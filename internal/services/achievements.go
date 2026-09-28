package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AchievementService struct {
	pool        *pgxpool.Pool
	r2APIToken  string
	r2AccountID string
	r2Bucket    string
}

func NewAchievementService(pool *pgxpool.Pool, r2Token, r2Account, r2Bucket string) *AchievementService {
	return &AchievementService{
		pool:        pool,
		r2APIToken:  r2Token,
		r2AccountID: r2Account,
		r2Bucket:    r2Bucket,
	}
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

// UploadFile uploads a certificate file to Cloudflare R2 and updates the
// achievement's file_key. If the achievement already has a different file_key,
// the old object is deleted on a best-effort basis.
func (s *AchievementService) UploadFile(ctx context.Context, id uuid.UUID, contentType string, body []byte) (*models.Achievement, error) {
	// Validate content type.
	ext, ok := contentTypeToExt[contentType]
	if !ok {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
	// Enforce 10 MB limit (body already limited by caller).
	const maxBytes = 10 << 20
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", len(body), maxBytes)
	}
	if s.r2APIToken == "" || s.r2AccountID == "" || s.r2Bucket == "" {
		return nil, fmt.Errorf("R2 storage not configured")
	}

	// Fetch current achievement to check existing file_key.
	var oldFileKey string
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(file_key, '') FROM achievements.achievements WHERE id = $1`, id,
	).Scan(&oldFileKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("fetching achievement: %w", err)
	}

	// Build object key: certificates/{id}/{id}.{ext}
	objectKey := fmt.Sprintf("certificates/%s/%s.%s", id.String(), id.String(), ext)

	// Upload to R2 via Cloudflare API (PUT).
	uploadURL := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/r2/buckets/%s/objects/%s",
		s.r2AccountID, s.r2Bucket, objectKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.r2APIToken)
	req.Header.Set("Content-Type", contentType)

	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("uploading to R2: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var cfErr struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&cfErr)
		detail := resp.Status
		if len(cfErr.Errors) > 0 {
			detail = cfErr.Errors[0].Message
		}
		return nil, fmt.Errorf("R2 upload failed (%s): %s", resp.Status, detail)
	}

	// Delete old object if it differs (best-effort).
	if oldFileKey != "" && oldFileKey != objectKey {
		go func(key string) {
			delURL := fmt.Sprintf(
				"https://api.cloudflare.com/client/v4/accounts/%s/r2/buckets/%s/objects/%s",
				s.r2AccountID, s.r2Bucket, key,
			)
			delReq, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, delURL, nil)
			delReq.Header.Set("Authorization", "Bearer "+s.r2APIToken)
			delClient := &http.Client{Timeout: 30 * time.Second}
			delResp, err := delClient.Do(delReq)
			if err == nil {
				delResp.Body.Close()
			}
		}(oldFileKey)
	}

	// Update file_key in database.
	var a models.Achievement
	err = s.pool.QueryRow(ctx,
		`UPDATE achievements.achievements
		 SET file_key = $2, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, title, organizer, date, type, drive_file_id, file_key,
		           certificate_number, participant_as, description, valid_until,
		           order_index, created_at`,
		id, objectKey,
	).Scan(&a.ID, &a.Title, &a.Organizer, &a.Date, &a.Type, &a.DriveFileID,
		&a.FileKey, &a.CertificateNumber, &a.ParticipantAs, &a.Description,
		&a.ValidUntil, &a.OrderIndex, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating file_key: %w", err)
	}
	return &a, nil
}

// contentTypeToExt maps allowed content types to file extensions.
var contentTypeToExt = map[string]string{
	"application/pdf": "pdf",
	"image/png":       "png",
	"image/jpeg":      "jpg",
	"image/webp":      "webp",
}

// nullableTime converts a *CustomDate to *time.Time for pgx scanning.
func nullableTime(d *models.CustomDate) interface{} {
	if d == nil {
		return nil
	}
	t := time.Time(*d)
	return &t
}
