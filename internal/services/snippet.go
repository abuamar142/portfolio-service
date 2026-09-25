package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SnippetService struct {
	pool *pgxpool.Pool
}

func NewSnippetService(pool *pgxpool.Pool) *SnippetService {
	return &SnippetService{pool: pool}
}

func (s *SnippetService) List(ctx context.Context, search, tag, language string, page, limit int) (*models.SnippetListResponse, error) {
	offset := (page - 1) * limit

	// Count total
	var countQuery string
	var args []any
	switch {
	case tag != "" && language != "":
		countQuery = `SELECT COUNT(DISTINCT sn.id) FROM snippets.snippets sn
			JOIN snippets.snippet_tags st ON st.snippet_id = sn.id
			JOIN snippets.tags t ON t.id = st.tag_id
			WHERE t.name = $1 AND sn.language = $2`
		args = append(args, tag, language)
	case tag != "":
		countQuery = `SELECT COUNT(DISTINCT st.snippet_id) FROM snippets.snippet_tags st JOIN snippets.tags t ON t.id = st.tag_id WHERE t.name = $1`
		args = append(args, tag)
	case language != "":
		countQuery = `SELECT COUNT(*) FROM snippets.snippets sn WHERE sn.language = $1`
		args = append(args, language)
	case search != "":
		countQuery = `SELECT COUNT(*) FROM snippets.snippets sn WHERE sn.title ILIKE '%' || $1 || '%' OR sn.description ILIKE '%' || $1 || '%' OR sn.code ILIKE '%' || $1 || '%'`
		args = append(args, search)
	default:
		countQuery = `SELECT COUNT(*) FROM snippets.snippets sn`
	}

	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting snippets: %w", err)
	}

	// Fetch with filters (parameterised — no sprintf injection)
	var query string
	var queryArgs []any
	queryArgs = append(queryArgs, limit, offset) // $1 = limit, $2 = offset
	paramIdx := 3
	var where []string
	if tag != "" {
		where = append(where, fmt.Sprintf(`sn.id IN (SELECT DISTINCT st.snippet_id FROM snippets.snippet_tags st JOIN snippets.tags t ON t.id = st.tag_id WHERE t.name = $%d)`, paramIdx))
		queryArgs = append(queryArgs, tag)
		paramIdx++
	}
	if language != "" {
		where = append(where, fmt.Sprintf(`sn.language = $%d`, paramIdx))
		queryArgs = append(queryArgs, language)
		paramIdx++
	}
	if search != "" {
		where = append(where, fmt.Sprintf(`(sn.title ILIKE '%%' || $%[1]d || '%%' OR sn.description ILIKE '%%' || $%[1]d || '%%' OR sn.code ILIKE '%%' || $%[1]d || '%%')`, paramIdx))
		queryArgs = append(queryArgs, search)
		paramIdx++
	}
	if len(where) > 0 {
		query = " WHERE " + strings.Join(where, " AND ")
	}
	query = fmt.Sprintf(`
		SELECT sn.id, sn.user_id, sn.title, sn.language, sn.code, sn.description, sn.created_at, sn.updated_at
		FROM snippets.snippets sn%s
		ORDER BY sn.created_at DESC LIMIT $1 OFFSET $2`, query)

	rows, err := s.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("listing snippets: %w", err)
	}
	defer rows.Close()

	snippets := []models.Snippet{}
	for rows.Next() {
		var sn models.Snippet
		if err := rows.Scan(&sn.ID, &sn.UserID, &sn.Title, &sn.Language, &sn.Code, &sn.Description, &sn.CreatedAt, &sn.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning snippet: %w", err)
		}
		sn.Tags, _ = s.getTags(ctx, sn.ID)
		snippets = append(snippets, sn)
	}
	return &models.SnippetListResponse{
		Snippets: snippets,
		Total:    total,
		Page:     page,
		Limit:    limit,
	}, nil
}

func (s *SnippetService) GetByID(ctx context.Context, id uuid.UUID) (*models.Snippet, error) {
	var sn models.Snippet
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, title, language, code, description, created_at, updated_at
		 FROM snippets.snippets WHERE id = $1`, id,
	).Scan(&sn.ID, &sn.UserID, &sn.Title, &sn.Language, &sn.Code, &sn.Description, &sn.CreatedAt, &sn.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting snippet: %w", err)
	}
	sn.Tags, _ = s.getTags(ctx, sn.ID)
	return &sn, nil
}

func (s *SnippetService) Create(ctx context.Context, userID uuid.UUID, req models.CreateSnippetRequest) (*models.Snippet, error) {
	var sn models.Snippet
	err := s.pool.QueryRow(ctx,
		`INSERT INTO snippets.snippets (user_id, title, language, code, description)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, user_id, title, language, code, description, created_at, updated_at`,
		userID, req.Title, req.Language, req.Code, req.Description,
	).Scan(&sn.ID, &sn.UserID, &sn.Title, &sn.Language, &sn.Code, &sn.Description, &sn.CreatedAt, &sn.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating snippet: %w", err)
	}

	if err := s.replaceTags(ctx, sn.ID, req.Tags); err != nil {
		return nil, err
	}
	sn.Tags, _ = s.getTags(ctx, sn.ID)
	return &sn, nil
}

func (s *SnippetService) Update(ctx context.Context, userID, snippetID uuid.UUID, req models.UpdateSnippetRequest) (*models.Snippet, error) {
	var sn models.Snippet
	err := s.pool.QueryRow(ctx,
		`UPDATE snippets.snippets SET title=$1, language=$2, code=$3, description=$4, updated_at=NOW()
		 WHERE id=$5 AND user_id=$6
		 RETURNING id, user_id, title, language, code, description, created_at, updated_at`,
		req.Title, req.Language, req.Code, req.Description, snippetID, userID,
	).Scan(&sn.ID, &sn.UserID, &sn.Title, &sn.Language, &sn.Code, &sn.Description, &sn.CreatedAt, &sn.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating snippet: %w", err)
	}

	if err := s.replaceTags(ctx, snippetID, req.Tags); err != nil {
		return nil, err
	}
	sn.Tags, _ = s.getTags(ctx, sn.ID)
	return &sn, nil
}

func (s *SnippetService) Delete(ctx context.Context, userID, snippetID uuid.UUID) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM snippets.snippets WHERE id = $1 AND user_id = $2`, snippetID, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting snippet: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("snippet not found or not owned by user")
	}
	return nil
}

// ListTags counts only tags actually used by snippets.
func (s *SnippetService) ListTags(ctx context.Context) ([]models.TagResponse, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.name, COUNT(st.snippet_id) as count
		 FROM snippets.tags t
		 JOIN snippets.snippet_tags st ON t.id = st.tag_id
		 GROUP BY t.id, t.name
		 ORDER BY count DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	defer rows.Close()
	tags := []models.TagResponse{}
	for rows.Next() {
		var t models.TagResponse
		if err := rows.Scan(&t.Tag, &t.Count); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, nil
}

// ListLanguages returns the distinct languages actually in use, with counts,
// so the UI filter only offers values that yield results.
func (s *SnippetService) ListLanguages(ctx context.Context) ([]models.LanguageResponse, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT language, COUNT(*) as count
		 FROM snippets.snippets
		 GROUP BY language
		 ORDER BY count DESC, language ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing languages: %w", err)
	}
	defer rows.Close()
	langs := []models.LanguageResponse{}
	for rows.Next() {
		var l models.LanguageResponse
		if err := rows.Scan(&l.Language, &l.Count); err != nil {
			return nil, fmt.Errorf("scanning language: %w", err)
		}
		langs = append(langs, l)
	}
	return langs, nil
}

func (s *SnippetService) replaceTags(ctx context.Context, snippetID uuid.UUID, tags []string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM snippets.snippet_tags WHERE snippet_id = $1`, snippetID); err != nil {
		return fmt.Errorf("clearing snippet tags: %w", err)
	}
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		var tagID int
		if err := s.pool.QueryRow(ctx,
			`INSERT INTO snippets.tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, tag,
		).Scan(&tagID); err != nil {
			return fmt.Errorf("upserting tag: %w", err)
		}
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO snippets.snippet_tags (snippet_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, snippetID, tagID,
		); err != nil {
			return fmt.Errorf("inserting snippet_tag: %w", err)
		}
	}
	return nil
}

func (s *SnippetService) getTags(ctx context.Context, snippetID uuid.UUID) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.name FROM snippets.tags t JOIN snippets.snippet_tags st ON t.id = st.tag_id WHERE st.snippet_id = $1`, snippetID)
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
