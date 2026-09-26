package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LinkService struct {
	pool *pgxpool.Pool
}

func NewLinkService(pool *pgxpool.Pool) *LinkService {
	return &LinkService{pool: pool}
}

func (s *LinkService) List(ctx context.Context, search string, tags []string, page, limit int) (*models.LinkListResponse, error) {
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
		countWhere = append(countWhere, fmt.Sprintf(`l.id IN (SELECT DISTINCT lt.link_id FROM links.link_tags lt JOIN links.tags t ON t.id = lt.tag_id WHERE t.name = ANY($%d))`, len(countArgs)))
	}
	if search != "" {
		countArgs = append(countArgs, search)
		countWhere = append(countWhere, fmt.Sprintf(`(l.title ILIKE '%%' || $%[1]d || '%%' OR l.description ILIKE '%%' || $%[1]d || '%%' OR l.url ILIKE '%%' || $%[1]d || '%%')`, len(countArgs)))
	}
	countQuery := `SELECT COUNT(*) FROM links.links l`
	if len(countWhere) > 0 {
		countQuery += " WHERE " + strings.Join(countWhere, " AND ")
	}

	var total int
	if err := s.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting links: %w", err)
	}

	// parameterised — no sprintf injection
	queryArgs := []any{limit, offset} // $1 = limit, $2 = offset
	var where []string
	if hasTag {
		queryArgs = append(queryArgs, tags)
		where = append(where, fmt.Sprintf(`l.id IN (SELECT DISTINCT lt.link_id FROM links.link_tags lt JOIN links.tags t ON t.id = lt.tag_id WHERE t.name = ANY($%d))`, len(queryArgs)))
	}
	if search != "" {
		queryArgs = append(queryArgs, search)
		where = append(where, fmt.Sprintf(`(l.title ILIKE '%%' || $%[1]d || '%%' OR l.description ILIKE '%%' || $%[1]d || '%%' OR l.url ILIKE '%%' || $%[1]d || '%%')`, len(queryArgs)))
	}
	fetchWhere := ""
	if len(where) > 0 {
		fetchWhere = " WHERE " + strings.Join(where, " AND ")
	}
	query := fmt.Sprintf(`
		SELECT l.id, l.user_id, l.url, l.title, l.description, l.created_at, l.updated_at
		FROM links.links l%s
		ORDER BY l.created_at DESC LIMIT $1 OFFSET $2`, fetchWhere)

	rows, err := s.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("listing links: %w", err)
	}
	defer rows.Close()

	links := []models.Link{}
	for rows.Next() {
		var l models.Link
		if err := rows.Scan(&l.ID, &l.UserID, &l.URL, &l.Title, &l.Description, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning link: %w", err)
		}
		l.Tags, _ = s.getTags(ctx, l.ID)
		links = append(links, l)
	}
	return &models.LinkListResponse{
		Links: links,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func (s *LinkService) GetByID(ctx context.Context, id uuid.UUID) (*models.Link, error) {
	var l models.Link
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, url, title, description, created_at, updated_at
		 FROM links.links WHERE id = $1`, id,
	).Scan(&l.ID, &l.UserID, &l.URL, &l.Title, &l.Description, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting link: %w", err)
	}
	l.Tags, _ = s.getTags(ctx, l.ID)
	return &l, nil
}

func (s *LinkService) Create(ctx context.Context, userID uuid.UUID, req models.CreateLinkRequest) (*models.Link, error) {
	var l models.Link
	err := s.pool.QueryRow(ctx,
		`INSERT INTO links.links (user_id, url, title, description)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, url, title, description, created_at, updated_at`,
		userID, req.URL, req.Title, req.Description,
	).Scan(&l.ID, &l.UserID, &l.URL, &l.Title, &l.Description, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating link: %w", err)
	}

	if err := s.replaceTags(ctx, l.ID, req.Tags); err != nil {
		return nil, err
	}
	l.Tags, _ = s.getTags(ctx, l.ID)
	return &l, nil
}

func (s *LinkService) Update(ctx context.Context, userID, linkID uuid.UUID, req models.UpdateLinkRequest) (*models.Link, error) {
	var l models.Link
	err := s.pool.QueryRow(ctx,
		`UPDATE links.links SET url=$1, title=$2, description=$3, updated_at=NOW()
		 WHERE id=$4 AND user_id=$5
		 RETURNING id, user_id, url, title, description, created_at, updated_at`,
		req.URL, req.Title, req.Description, linkID, userID,
	).Scan(&l.ID, &l.UserID, &l.URL, &l.Title, &l.Description, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating link: %w", err)
	}

	if err := s.replaceTags(ctx, linkID, req.Tags); err != nil {
		return nil, err
	}
	l.Tags, _ = s.getTags(ctx, l.ID)
	return &l, nil
}

// replaceTags rewrites the link's tag associations. Every error bubbles up:
// a swallowed failure here leaves stale junction rows behind the owner's
// back. Mirrors SnippetService.replaceTags (per-domain tables, same shape).
func (s *LinkService) replaceTags(ctx context.Context, linkID uuid.UUID, tags []string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM links.link_tags WHERE link_id = $1`, linkID); err != nil {
		return fmt.Errorf("clearing link tags: %w", err)
	}
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		var tagID int
		if err := s.pool.QueryRow(ctx,
			`INSERT INTO links.tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, tag,
		).Scan(&tagID); err != nil {
			return fmt.Errorf("upserting tag: %w", err)
		}
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO links.link_tags (link_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, linkID, tagID,
		); err != nil {
			return fmt.Errorf("inserting link_tag: %w", err)
		}
	}
	return nil
}

func (s *LinkService) Delete(ctx context.Context, userID, linkID uuid.UUID) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM links.links WHERE id = $1 AND user_id = $2`, linkID, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting link: %w", err)
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListTags counts only tags actually used by links — the tags table is shared
// with quotes, so an INNER JOIN keeps quote-only tags out of the filter pills.
func (s *LinkService) ListTags(ctx context.Context) ([]models.TagResponse, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.name, COUNT(lt.link_id) as count
		 FROM links.tags t
		 JOIN links.link_tags lt ON t.id = lt.tag_id
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

func (s *LinkService) getTags(ctx context.Context, linkID uuid.UUID) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.name FROM links.tags t JOIN links.link_tags lt ON t.id = lt.tag_id WHERE lt.link_id = $1`, linkID)
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
