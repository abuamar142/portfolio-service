package services

import (
	"context"
	"fmt"
	"time"

	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AchievementService struct {
	pool *pgxpool.Pool
}

func NewAchievementService(pool *pgxpool.Pool) *AchievementService {
	return &AchievementService{pool: pool}
}

func (s *AchievementService) Create(ctx context.Context, req models.CreateAchievementRequest) (*models.Achievement, error) {
	date, err := models.ParseCustomDate(req.Date)
	if err != nil {
		return nil, err
	}
	validUntil, err := models.ParseNullableDate(req.ValidUntil)
	if err != nil {
		return nil, err
	}

	var a models.Achievement
	err = s.pool.QueryRow(ctx,
		`INSERT INTO achievements.achievements
			(title, organizer, date, type, drive_file_id, certificate_number,
			 participant_as, description, valid_until, order_index)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, title, organizer, date, type, drive_file_id, file_key,
		           certificate_number, participant_as, description, valid_until,
		           order_index, created_at`,
		req.Title, req.Organizer, time.Time(date), req.Type, req.DriveFileID,
		req.CertificateNumber, req.ParticipantAs, req.Description, nullableTime(validUntil),
		req.OrderIndex,
	).Scan(&a.ID, &a.Title, &a.Organizer, &a.Date, &a.Type, &a.DriveFileID,
		&a.FileKey, &a.CertificateNumber, &a.ParticipantAs, &a.Description,
		&a.ValidUntil, &a.OrderIndex, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating achievement: %w", err)
	}
	return &a, nil
}

func (s *AchievementService) List(ctx context.Context) (*models.AchievementListResponse, error) {
	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM achievements.achievements`,
	).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting achievements: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, title, organizer, date, type, drive_file_id, file_key,
		        certificate_number, participant_as, description, valid_until,
		        order_index, created_at
		 FROM achievements.achievements
		 ORDER BY order_index ASC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing achievements: %w", err)
	}
	defer rows.Close()

	items := []models.Achievement{} // non-nil: JSON [] instead of null
	for rows.Next() {
		var a models.Achievement
		if err := rows.Scan(&a.ID, &a.Title, &a.Organizer, &a.Date, &a.Type,
			&a.DriveFileID, &a.FileKey, &a.CertificateNumber, &a.ParticipantAs,
			&a.Description, &a.ValidUntil, &a.OrderIndex, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning achievement: %w", err)
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating achievements: %w", err)
	}

	return &models.AchievementListResponse{Achievements: items, Total: total}, nil
}

func (s *AchievementService) Update(ctx context.Context, id uuid.UUID, req models.UpdateAchievementRequest) (*models.Achievement, error) {
	date, err := models.ParseCustomDate(req.Date)
	if err != nil {
		return nil, err
	}
	validUntil, err := models.ParseNullableDate(req.ValidUntil)
	if err != nil {
		return nil, err
	}

	var a models.Achievement
	err = s.pool.QueryRow(ctx,
		`UPDATE achievements.achievements SET
			title = $2, organizer = $3, date = $4, type = $5, drive_file_id = $6,
			certificate_number = $7, participant_as = $8, description = $9,
			valid_until = $10, order_index = $11, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, title, organizer, date, type, drive_file_id, file_key,
		           certificate_number, participant_as, description, valid_until,
		           order_index, created_at`,
		id, req.Title, req.Organizer, time.Time(date), req.Type, req.DriveFileID,
		req.CertificateNumber, req.ParticipantAs, req.Description, nullableTime(validUntil),
		req.OrderIndex,
	).Scan(&a.ID, &a.Title, &a.Organizer, &a.Date, &a.Type, &a.DriveFileID,
		&a.FileKey, &a.CertificateNumber, &a.ParticipantAs, &a.Description,
		&a.ValidUntil, &a.OrderIndex, &a.CreatedAt)
	if err != nil {
		return nil, err // pgx.ErrNoRows bubbles up for the handler's 404
	}
	return &a, nil
}

func (s *AchievementService) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM achievements.achievements WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting achievement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// nullableTime converts a *CustomDate to *time.Time for pgx scanning.
func nullableTime(d *models.CustomDate) interface{} {
	if d == nil {
		return nil
	}
	t := time.Time(*d)
	return &t
}
