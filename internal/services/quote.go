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

func (s *QuoteService) List(ctx context.Context, search string, tags []string, page, limit int) (*models.QuoteListResponse, error) {
	offset := (page - 1) * limit
	hasTag := len(tags) > 0

	// Count shares the condition list built the same way as the fetch below,
	// so the total can never disagree with the rows for any tag/search mix.
	// (The old four-branch switch bound a []string to `t.name = $1` and
	// failed with an encode error whenever tag and search came together.)
	var countArgs []any
	var countWhere []string
	if hasTag {
		countArgs = append(countArgs, tags)
		countWhere = append(countWhere, fmt.Sprintf(`q.id IN (SELECT DISTINCT qt.quote_id FROM quotes.quote_tags qt JOIN quotes.tags t ON t.id = qt.tag_id WHERE t.name = ANY($%d))`, len(countArgs)))
	}
	if search != "" {
		countArgs = append(countArgs, search)
		countWhere = append(countWhere, fmt.Sprintf(`q.content ILIKE '%%' || $%[1]d || '%%'`, len(countArgs)))
	}
	countQuery := `SELECT COUNT(*) FROM quotes.quotes q`
	if len(countWhere) > 0 {
		countQuery += " WHERE " + strings.Join(countWhere, " AND ")
	}

	var total int
	if err := s.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting quotes: %w", err)
	}

	// parameterised — no sprintf injection
	queryArgs := []any{limit, offset} // $1 = limit, $2 = offset
	var where []string
	if hasTag {
		queryArgs = append(queryArgs, tags)
		where = append(where, fmt.Sprintf(`q.id IN (SELECT DISTINCT qt.quote_id FROM quotes.quote_tags qt JOIN quotes.tags t ON t.id = qt.tag_id WHERE t.name = ANY($%d))`, len(queryArgs)))
	}
	if search != "" {
		queryArgs = append(queryArgs, search)
		where = append(where, fmt.Sprintf(`q.content ILIKE '%%' || $%[1]d || '%%'`, len(queryArgs)))
	}
	fetchWhere := ""
	if len(where) > 0 {
		fetchWhere = " WHERE " + strings.Join(where, " AND ")
	}
	query := fmt.Sprintf(`
		SELECT q.id, q.user_id, q.content, q.author_name, q.is_anonymous, q.source, q.color, q.created_at, q.updated_at
		FROM quotes.quotes q%s
		ORDER BY random() LIMIT $1 OFFSET $2`, fetchWhere)

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

	if err := s.replaceTags(ctx, q.ID, req.Tags); err != nil {
		return nil, err
	}
	q.Tags, _ = s.getTags(ctx, q.ID)
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

	if err := s.replaceTags(ctx, quoteID, req.Tags); err != nil {
		return nil, err
	}
	q.Tags, _ = s.getTags(ctx, q.ID)
	return &q, nil
}

// replaceTags rewrites the quote's tag associations. Every error bubbles up:
// a swallowed failure here leaves stale junction rows behind the owner's
// back. Mirrors SnippetService.replaceTags (per-domain tables, same shape).
func (s *QuoteService) replaceTags(ctx context.Context, quoteID uuid.UUID, tags []string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM quotes.quote_tags WHERE quote_id = $1`, quoteID); err != nil {
		return fmt.Errorf("clearing quote tags: %w", err)
	}
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		var tagID int
		if err := s.pool.QueryRow(ctx,
			`INSERT INTO quotes.tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, tag,
		).Scan(&tagID); err != nil {
			return fmt.Errorf("upserting tag: %w", err)
		}
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO quotes.quote_tags (quote_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, quoteID, tagID,
		); err != nil {
			return fmt.Errorf("inserting quote_tag: %w", err)
		}
	}
	return nil
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
