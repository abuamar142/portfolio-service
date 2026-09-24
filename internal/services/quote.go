package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QuoteService struct {
	pool *pgxpool.Pool
}

func NewQuoteService(pool *pgxpool.Pool) *QuoteService {
	return &QuoteService{pool: pool}
}

func (s *QuoteService) List(ctx context.Context, search, tag string, page, limit int) (*models.QuoteListResponse, error) {
	offset := (page - 1) * limit

	// Count total
	var countQuery string
	var args []any
	if tag != "" {
		countQuery = `SELECT COUNT(DISTINCT qt.quote_id) FROM quotes.quote_tags qt JOIN quotes.tags t ON t.id = qt.tag_id WHERE t.name = $1`
		args = append(args, tag)
		if search != "" {
			countQuery = `SELECT COUNT(DISTINCT q.id) FROM quotes.quotes q JOIN quotes.quote_tags qt ON qt.quote_id = q.id JOIN quotes.tags t ON t.id = qt.tag_id WHERE t.name = $1 AND q.content ILIKE '%' || $2 || '%'`
			args = append(args, search)
		}
	} else if search != "" {
		countQuery = `SELECT COUNT(*) FROM quotes.quotes q WHERE q.content ILIKE '%' || $1 || '%'`
		args = append(args, search)
	} else {
		countQuery = `SELECT COUNT(*) FROM quotes.quotes q`
	}

	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting quotes: %w", err)
	}

	// Fetch quotes with search/tag filters (parameterised — no sprintf injection)
	var query string
	var queryArgs []any
	queryArgs = append(queryArgs, limit, offset) // $1 = limit, $2 = offset
	paramIdx := 3
	if tag != "" {
		query += fmt.Sprintf(` WHERE q.id IN (SELECT DISTINCT qt.quote_id FROM quotes.quote_tags qt JOIN quotes.tags t ON t.id = qt.tag_id WHERE t.name = $%d)`, paramIdx)
		queryArgs = append(queryArgs, tag)
		paramIdx++
	}
	if search != "" {
		if tag != "" {
			query += fmt.Sprintf(` AND q.content ILIKE '%%' || $%d || '%%'`, paramIdx)
		} else {
			query += fmt.Sprintf(` WHERE q.content ILIKE '%%' || $%d || '%%'`, paramIdx)
		}
		queryArgs = append(queryArgs, search)
		paramIdx++
	}
	query = fmt.Sprintf(`
		SELECT q.id, q.user_id, q.content, q.author_name, q.is_anonymous, q.source, q.color, q.created_at, q.updated_at
		FROM quotes.quotes q%s
		ORDER BY random() LIMIT $1 OFFSET $2`, query)

	rows, err := s.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("listing quotes: %w", err)
	}
	defer rows.Close()

	var quotes []models.Quote
	for rows.Next() {
		var q models.Quote
		if err := rows.Scan(&q.ID, &q.UserID, &q.Content, &q.AuthorName, &q.IsAnonymous, &q.Source, &q.Color, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning quote: %w", err)
		}
		q.Tags, _ = s.getTags(ctx, q.ID)
		quotes = append(quotes, q)
	}
	return &models.QuoteListResponse{
		Quotes: quotes,
		Total:  total,
		Page:   page,
		Limit:  limit,
	}, nil
}

func (s *QuoteService) GetByID(ctx context.Context, id uuid.UUID) (*models.Quote, error) {
	var q models.Quote
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, content, author_name, is_anonymous, source, color, created_at, updated_at
		 FROM quotes.quotes WHERE id = $1`, id,
	).Scan(&q.ID, &q.UserID, &q.Content, &q.AuthorName, &q.IsAnonymous, &q.Source, &q.Color, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting quote: %w", err)
	}
	q.Tags, _ = s.getTags(ctx, q.ID)
	return &q, nil
}

func (s *QuoteService) Create(ctx context.Context, userID uuid.UUID, req models.CreateQuoteRequest) (*models.Quote, error) {
	colors := []string{"yellow", "pink", "blue", "green", "white"}
	color := colors[uuid.New().ID()%5]

	var q models.Quote
	err := s.pool.QueryRow(ctx,
		`INSERT INTO quotes.quotes (user_id, content, author_name, is_anonymous, source, color)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, content, author_name, is_anonymous, source, color, created_at, updated_at`,
		userID, req.Content, req.AuthorName, req.IsAnonymous, req.Source, color,
	).Scan(&q.ID, &q.UserID, &q.Content, &q.AuthorName, &q.IsAnonymous, &q.Source, &q.Color, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating quote: %w", err)
	}

	// Insert tags
	for _, tag := range req.Tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		// Upsert tag to tags table
		var tagID int
		err := s.pool.QueryRow(ctx,
			`INSERT INTO quotes.tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, tag,
		).Scan(&tagID)
		if err != nil {
			return nil, fmt.Errorf("upserting tag: %w", err)
		}
		// Insert quote_tags
		_, err = s.pool.Exec(ctx,
			`INSERT INTO quotes.quote_tags (quote_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, q.ID, tagID,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting quote_tag: %w", err)
		}
	}
	q.Tags = req.Tags
	return &q, nil
}

func (s *QuoteService) Update(ctx context.Context, userID, quoteID uuid.UUID, req models.UpdateQuoteRequest) (*models.Quote, error) {
	var q models.Quote
	err := s.pool.QueryRow(ctx,
		`UPDATE quotes.quotes SET content=$1, author_name=$2, is_anonymous=$3, source=$4, updated_at=NOW()
		 WHERE id=$5 AND user_id=$6
		 RETURNING id, user_id, content, author_name, is_anonymous, source, color, created_at, updated_at`,
		req.Content, req.AuthorName, req.IsAnonymous, req.Source, quoteID, userID,
	).Scan(&q.ID, &q.UserID, &q.Content, &q.AuthorName, &q.IsAnonymous, &q.Source, &q.Color, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating quote: %w", err)
	}

	// Replace tags
	s.pool.Exec(ctx, `DELETE FROM quotes.quote_tags WHERE quote_id = $1`, quoteID)
	for _, tag := range req.Tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		// Upsert tag
		var tagID int
		err := s.pool.QueryRow(ctx,
			`INSERT INTO quotes.tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, tag,
		).Scan(&tagID)
		if err != nil {
			return nil, fmt.Errorf("upserting tag: %w", err)
		}
		// Insert quote_tags
		s.pool.Exec(ctx, `INSERT INTO quotes.quote_tags (quote_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, quoteID, tagID)
	}
	q.Tags = req.Tags
	return &q, nil
}

func (s *QuoteService) Delete(ctx context.Context, userID, quoteID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM quotes.quotes WHERE id = $1 AND user_id = $2`, quoteID, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting quote: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("quote not found or not owned by user")
	}
	return nil
}

func (s *QuoteService) ListTags(ctx context.Context) ([]models.TagResponse, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.name, COUNT(qt.quote_id) as count
		 FROM quotes.tags t
		 LEFT JOIN quotes.quote_tags qt ON t.id = qt.tag_id
		 GROUP BY t.id, t.name
		 ORDER BY count DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	defer rows.Close()
	var tags []models.TagResponse
	for rows.Next() {
		var t models.TagResponse
		if err := rows.Scan(&t.Tag, &t.Count); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *QuoteService) getTags(ctx context.Context, quoteID uuid.UUID) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.name FROM quotes.tags t JOIN quotes.quote_tags qt ON t.id = qt.tag_id WHERE qt.quote_id = $1`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}
