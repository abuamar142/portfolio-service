package services

import (
	"context"
	"fmt"

	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FeedbackService struct {
	pool *pgxpool.Pool
}

func NewFeedbackService(pool *pgxpool.Pool) *FeedbackService {
	return &FeedbackService{pool: pool}
}

func (s *FeedbackService) Create(ctx context.Context, req models.CreateFeedbackRequest) (*models.Feedback, error) {
	var f models.Feedback
	err := s.pool.QueryRow(ctx,
		`INSERT INTO feedback.feedback (message, contact, page_url)
		 VALUES ($1, $2, $3)
		 RETURNING id, message, contact, page_url, status, created_at`,
		req.Message, req.Contact, req.PageURL,
	).Scan(&f.ID, &f.Message, &f.Contact, &f.PageURL, &f.Status, &f.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating feedback: %w", err)
	}
	return &f, nil
}

// List returns newest-first feedback; status must already be validated by the
// handler ("", "all", "new" or "read") so it is interpolated as data, never
// as a bind parameter position.
func (s *FeedbackService) List(ctx context.Context, status string) (*models.FeedbackListResponse, error) {
	where := ""
	var args []any
	if status == "new" || status == "read" {
		where = " WHERE status = $1"
		args = append(args, status)
	}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM feedback.feedback`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting feedback: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, message, contact, page_url, status, created_at
		 FROM feedback.feedback`+where+` ORDER BY created_at DESC, id DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("listing feedback: %w", err)
	}
	defer rows.Close()

	items := []models.Feedback{} // non-nil: JSON [] instead of null
	for rows.Next() {
		var f models.Feedback
		if err := rows.Scan(&f.ID, &f.Message, &f.Contact, &f.PageURL, &f.Status, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning feedback: %w", err)
		}
		items = append(items, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating feedback: %w", err)
	}

	return &models.FeedbackListResponse{Feedback: items, Total: total}, nil
}

func (s *FeedbackService) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*models.Feedback, error) {
	var f models.Feedback
	err := s.pool.QueryRow(ctx,
		`UPDATE feedback.feedback SET status = $1 WHERE id = $2
		 RETURNING id, message, contact, page_url, status, created_at`,
		status, id,
	).Scan(&f.ID, &f.Message, &f.Contact, &f.PageURL, &f.Status, &f.CreatedAt)
	if err != nil {
		return nil, err // pgx.ErrNoRows bubbles up for the handler's 404
	}
	return &f, nil
}

func (s *FeedbackService) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM feedback.feedback WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting feedback: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
