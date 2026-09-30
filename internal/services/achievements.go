package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fileStore is the row access the file paths need: read the current key, write
// the new one. A narrow interface rather than the pool so the ordering rules
// can be tested without a database — the interesting failure is "the row write
// failed after the upload succeeded", which is impossible to provoke against a
// real pool. *pgxpool-backed achievementStore satisfies it.
type fileStore interface {
	// currentFileKey returns the key the achievement points at, or "".
	// pgx.ErrNoRows when the achievement does not exist.
	currentFileKey(ctx context.Context, id uuid.UUID) (string, error)
	// setFileMetadata writes the file columns; an empty key clears them.
	setFileMetadata(ctx context.Context, id uuid.UUID, key string, fileName *string, fileSize int64) (*models.Achievement, error)
}

type AchievementService struct {
	pool *pgxpool.Pool
	// media owns the R2 credentials and the portfolio-assets bucket. This
	// service holds no storage credential at all.
	media *MediaClient
	// store is the row access the file paths use. Defaults to this service's
	// own pool-backed implementation; a test substitutes a fake.
	store fileStore
}

func NewAchievementService(pool *pgxpool.Pool, media *MediaClient) *AchievementService {
	s := &AchievementService{pool: pool, media: media}
	s.store = &pgAchievementStore{pool: pool}
	return s
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
			(title, organizer, date, type, certificate_number,
			 participant_as, description, valid_until, order_index)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, title, organizer, date, type,
		           COALESCE(file_key, ''), file_name, file_size,
		           certificate_number, participant_as, description, valid_until,
		           order_index, created_at`,
		req.Title, req.Organizer, time.Time(date), req.Type,
		req.CertificateNumber, req.ParticipantAs, req.Description, nullableTime(validUntil),
		req.OrderIndex,
	).Scan(&a.ID, &a.Title, &a.Organizer, &a.Date, &a.Type,
		&a.FileKey, &a.FileName, &a.FileSize,
		&a.CertificateNumber, &a.ParticipantAs, &a.Description,
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
		`SELECT id, title, organizer, date, type,
		        COALESCE(file_key, ''), file_name, file_size,
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
			&a.FileKey, &a.FileName, &a.FileSize,
			&a.CertificateNumber, &a.ParticipantAs,
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
			title = $2, organizer = $3, date = $4, type = $5,
			certificate_number = $6, participant_as = $7, description = $8,
			valid_until = $9, order_index = $10, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, title, organizer, date, type,
		           COALESCE(file_key, ''), file_name, file_size,
		           certificate_number, participant_as, description, valid_until,
		           order_index, created_at`,
		id, req.Title, req.Organizer, time.Time(date), req.Type,
		req.CertificateNumber, req.ParticipantAs, req.Description, nullableTime(validUntil),
		req.OrderIndex,
	).Scan(&a.ID, &a.Title, &a.Organizer, &a.Date, &a.Type,
		&a.FileKey, &a.FileName, &a.FileSize,
		&a.CertificateNumber, &a.ParticipantAs, &a.Description,
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

// UploadFile stores a certificate file via media-service and records its key
// on the achievement.
//
// The ordering is deliberate, and it matches the other callers of
// media-service:
//
//  1. Validate the type and read the current row first, so a request that can
//     never succeed fails before anything is uploaded — a rejected request
//     must not leave an orphan in the bucket.
//  2. Upload, which returns the new key.
//  3. Write the key to the row.
//  4. Only then remove the object it replaced.
//
// Reversing 3 and 4 would leave the row pointing at a file that no longer
// exists if the write failed. If step 3 fails after a successful upload, the
// new object is removed instead of being left as an orphan.
//
// The key is stored, not the URL: the dashboard builds the link itself from
// FILES_URL and the key. Storing the URL would double the prefix.
func (s *AchievementService) UploadFile(ctx context.Context, token string, id uuid.UUID, contentType string, body []byte, fileName *string, fileSize int64) (*models.Achievement, error) {
	// The type is checked here as well as by media-service, which sniffs the
	// bytes. This check gives the dashboard its specific message ("content
	// type must be one of: …") for a type we never accept, before a body is
	// read; media-service is still the authority on what is actually stored.
	if _, ok := contentTypeToExt[contentType]; !ok {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
	const maxBytes = 10 << 20
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", len(body), maxBytes)
	}

	// 1. Resolve the row and the key it currently points at.
	oldFileKey, err := s.store.currentFileKey(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("fetching achievement: %w", err)
	}

	// 2. Upload. The key comes back shaped as
	// certificates/{id}/file-{epoch}.{ext}.
	_, newKey, err := s.media.Upload(ctx, token, "certificates", id.String(), "file", body)
	if err != nil {
		return nil, err
	}

	// 3. Persist.
	a, err := s.store.setFileMetadata(ctx, id, newKey, fileName, fileSize)
	if err != nil {
		// The upload succeeded but the row was not updated: remove the new
		// object rather than leaving a file nothing points at. A failure here
		// is reported alongside the row error — the row error is the one that
		// matters, but an orphan is worth naming.
		if delErr := s.media.Delete(ctx, token, newKey); delErr != nil {
			return nil, fmt.Errorf("updating row after upload (orphan %s left behind: %v): %w", newKey, delErr, err)
		}
		return nil, err
	}

	// 4. The row is correct now. A stale object costs storage but breaks
	// nothing, so a failure to remove it must not fail the upload the user
	// just made.
	//
	// Detached from the request context: the caller should not wait on a
	// cleanup call, and a client that hangs up must not cancel it.
	if oldFileKey != "" && oldFileKey != newKey {
		go func(key string) {
			if delErr := s.media.Delete(context.WithoutCancel(ctx), token, key); delErr != nil {
				log.Printf("media: could not remove replaced certificate (%s): %v", key, delErr)
			}
		}(oldFileKey)
	}

	return a, nil
}

// DeleteFile clears the file metadata for an achievement and removes the
// object.
//
// The row is cleared first and the object removed after, for the same reason
// as an upload: a row pointing at a deleted file is worse than an object
// nothing points at. Unlike the replace path, the delete here runs
// synchronously — the request's whole purpose is to remove the file, and
// reporting success while it survives would be a lie.
func (s *AchievementService) DeleteFile(ctx context.Context, token string, id uuid.UUID) (*models.Achievement, error) {
	fileKey, err := s.store.currentFileKey(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("fetching achievement: %w", err)
	}

	a, err := s.store.setFileMetadata(ctx, id, "", nil, 0)
	if err != nil {
		return nil, err
	}

	if fileKey != "" {
		if err := s.media.Delete(ctx, token, fileKey); err != nil {
			// The column is already clear, so the next upload will not
			// reference this object — but it is still in the bucket. Reported
			// rather than swallowed, because the caller asked for it gone.
			return a, fmt.Errorf("clearing the column succeeded but removing the object failed: %w", err)
		}
	}

	return a, nil
}

// setFileMetadata writes the file columns and returns the updated row.
//
// An empty key clears the columns (file_key/file_name/file_size NULL), which
// is what DeleteFile needs; an upload passes the key it just stored. One
// helper because the two paths must produce the same row shape — the handler
// returns it straight to the dashboard.
// pgAchievementStore is the production fileStore, backed by the pool.
type pgAchievementStore struct{ pool *pgxpool.Pool }

func (s *pgAchievementStore) currentFileKey(ctx context.Context, id uuid.UUID) (string, error) {
	var key string
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(file_key, '') FROM achievements.achievements WHERE id = $1`, id,
	).Scan(&key)
	if err != nil {
		return "", err
	}
	return key, nil
}

func (s *pgAchievementStore) setFileMetadata(ctx context.Context, id uuid.UUID, key string, fileName *string, fileSize int64) (*models.Achievement, error) {
	var (
		a   models.Achievement
		err error
	)
	if key == "" {
		err = s.pool.QueryRow(ctx,
			`UPDATE achievements.achievements
			 SET file_key = NULL, file_name = NULL, file_size = NULL, updated_at = NOW()
			 WHERE id = $1
			 RETURNING id, title, organizer, date, type,
			           COALESCE(file_key, ''), file_name, file_size,
			           certificate_number, participant_as, description, valid_until,
			           order_index, created_at`,
			id,
		).Scan(&a.ID, &a.Title, &a.Organizer, &a.Date, &a.Type,
			&a.FileKey, &a.FileName, &a.FileSize,
			&a.CertificateNumber, &a.ParticipantAs, &a.Description,
			&a.ValidUntil, &a.OrderIndex, &a.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("clearing file metadata: %w", err)
		}
		return &a, nil
	}

	err = s.pool.QueryRow(ctx,
		`UPDATE achievements.achievements
		 SET file_key = $2, file_name = $3, file_size = $4, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, title, organizer, date, type,
		           COALESCE(file_key, ''), file_name, file_size,
		           certificate_number, participant_as, description, valid_until,
		           order_index, created_at`,
		id, key, fileName, fileSize,
	).Scan(&a.ID, &a.Title, &a.Organizer, &a.Date, &a.Type,
		&a.FileKey, &a.FileName, &a.FileSize,
		&a.CertificateNumber, &a.ParticipantAs, &a.Description,
		&a.ValidUntil, &a.OrderIndex, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating file metadata: %w", err)
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
