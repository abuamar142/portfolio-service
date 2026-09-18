package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/abuamar142/quote-service/internal/models"
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
	countQuery := `SELECT COUNT(DISTINCT q.id) FROM quotes q`
	args := []any{}
	where := []string{}

	if search != "" {
		where = append(where, fmt.Sprintf("q.content ILIKE '%%%s%%'", search))
	}
	if tag != "" {
		countQuery = `SELECT COUNT(DISTINCT q.id) FROM quotes q JOIN quote_tags qt ON qt.quote_id = q.id`
		where = append(where, fmt.Sprintf("qt.tag = '%s'", tag))
	}
	if len(where) > 0 {
		countQuery += " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting quotes: %w", err)
	}

	// Fetch quotes
	query := `
		SELECT DISTINCT q.id, q.user_id, q.content, q.author_name, q.is_anonymous, q.source, q.color, q.created_at, q.updated_at
		FROM quotes q`
	if tag != "" {
		query += ` JOIN quote_tags qt ON qt.quote_id = q.id`
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += ` ORDER BY random() LIMIT $1 OFFSET $2`

	rows, err := s.pool.Query(ctx, query, limit, offset)
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
		 FROM quotes WHERE id = $1`, id,
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
		`INSERT INTO quotes (user_id, content, author_name, is_anonymous, source, color)
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
		_, err := s.pool.Exec(ctx,
			`INSERT INTO quote_tags (quote_id, tag) VALUES ($1, $2)`, q.ID, tag,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting tag: %w", err)
		}
	}

	q.Tags = req.Tags
	return &q, nil
}

func (s *QuoteService) Update(ctx context.Context, userID, quoteID uuid.UUID, req models.UpdateQuoteRequest) (*models.Quote, error) {
	var q models.Quote
	err := s.pool.QueryRow(ctx,
		`UPDATE quotes SET content=$1, author_name=$2, is_anonymous=$3, source=$4, updated_at=NOW()
		 WHERE id=$5 AND user_id=$6
		 RETURNING id, user_id, content, author_name, is_anonymous, source, color, created_at, updated_at`,
		req.Content, req.AuthorName, req.IsAnonymous, req.Source, quoteID, userID,
	).Scan(&q.ID, &q.UserID, &q.Content, &q.AuthorName, &q.IsAnonymous, &q.Source, &q.Color, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating quote: %w", err)
	}

	// Replace tags
	s.pool.Exec(ctx, `DELETE FROM quote_tags WHERE quote_id = $1`, quoteID)
	for _, tag := range req.Tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		s.pool.Exec(ctx, `INSERT INTO quote_tags (quote_id, tag) VALUES ($1, $2)`, quoteID, tag)
	}

	q.Tags = req.Tags
	return &q, nil
}

func (s *QuoteService) Delete(ctx context.Context, userID, quoteID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM quotes WHERE id = $1 AND user_id = $2`, quoteID, userID,
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
		`SELECT tag, COUNT(*) as count FROM quote_tags GROUP BY tag ORDER BY count DESC`)
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
	rows, err := s.pool.Query(ctx, `SELECT tag FROM quote_tags WHERE quote_id = $1`, quoteID)
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
