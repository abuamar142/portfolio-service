package models

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Achievement is a portfolio certificate, seminar, webinar, or similar
// credential. Date fields marshal as "YYYY-MM-DD" in JSON and valid_until
// is nullable (null when not applicable).
type Achievement struct {
	ID                uuid.UUID  `json:"id"`
	Title             string     `json:"title"`
	Organizer         string     `json:"organizer"`
	Date              CustomDate `json:"date"`
	Type              string     `json:"type"`
	DriveFileID       string     `json:"drive_file_id"`
	FileKey           string     `json:"file_key"`
	CertificateNumber string     `json:"certificate_number"`
	ParticipantAs     string     `json:"participant_as"`
	Description       string     `json:"description"`
	ValidUntil        *CustomDate `json:"valid_until"`
	OrderIndex        int        `json:"order_index"`
	CreatedAt         time.Time  `json:"created_at"`
}

type CreateAchievementRequest struct {
	Title             string  `json:"title"`
	Organizer         string  `json:"organizer"`
	Date              string  `json:"date"`
	Type              string  `json:"type"`
	DriveFileID       string  `json:"drive_file_id"`
	CertificateNumber string  `json:"certificate_number"`
	ParticipantAs     string  `json:"participant_as"`
	Description       string  `json:"description"`
	ValidUntil        *string `json:"valid_until"`
	OrderIndex        int     `json:"order_index"`
}

type UpdateAchievementRequest struct {
	Title             string  `json:"title"`
	Organizer         string  `json:"organizer"`
	Date              string  `json:"date"`
	Type              string  `json:"type"`
	DriveFileID       string  `json:"drive_file_id"`
	CertificateNumber string  `json:"certificate_number"`
	ParticipantAs     string  `json:"participant_as"`
	Description       string  `json:"description"`
	ValidUntil        *string `json:"valid_until"`
	OrderIndex        int     `json:"order_index"`
}

type AchievementListResponse struct {
	Achievements []Achievement `json:"achievements"`
	Total        int           `json:"total"`
}

// CustomDate wraps time.Time to marshal as "YYYY-MM-DD" instead of RFC3339.
type CustomDate time.Time

func (d CustomDate) MarshalJSON() ([]byte, error) {
	t := time.Time(d)
	if t.IsZero() {
		return json.Marshal(nil)
	}
	return json.Marshal(t.Format("2006-01-02"))
}

func (d *CustomDate) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("date must be YYYY-MM-DD: %w", err)
	}
	*d = CustomDate(t)
	return nil
}

// ParseCustomDate parses a "YYYY-MM-DD" string into a CustomDate.
func ParseCustomDate(s string) (CustomDate, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return CustomDate{}, fmt.Errorf("date must be YYYY-MM-DD: %w", err)
	}
	return CustomDate(t), nil
}

// ParseNullableDate parses a potentially empty/nil date string. Empty string
// or nil pointer returns nil.
func ParseNullableDate(s *string) (*CustomDate, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	d, err := ParseCustomDate(*s)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// pgEpoch is PostgreSQL's binary DATE epoch (days since2000-01-01).
var pgEpoch = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// Scan implements sql.Scanner (and pgtype's twin interface) so DATE columns
// load without a cast: pgx may hand us text ("YYYY-MM-DD"), the binary day
// count since2000-01-01, a time.Time, or plain int days.
func (d *CustomDate) Scan(src any) error {
	if src == nil {
		*d = CustomDate{}
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		*d = CustomDate(v)
		return nil
	case string:
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return fmt.Errorf("date must be YYYY-MM-DD: %w", err)
		}
		*d = CustomDate(t)
		return nil
	case []byte:
		if len(v) == 4 { // binary DATE: int32 days since the PG epoch, big-endian
			*d = CustomDate(pgEpoch.AddDate(0, 0, int(int32(binary.BigEndian.Uint32(v)))))
			return nil
		}
		t, err := time.Parse("2006-01-02", string(v))
		if err != nil {
			return fmt.Errorf("date must be YYYY-MM-DD: %w", err)
		}
		*d = CustomDate(t)
		return nil
	case int32:
		*d = CustomDate(pgEpoch.AddDate(0, 0, int(v)))
		return nil
	case int64:
		*d = CustomDate(pgEpoch.AddDate(0, 0, int(v)))
		return nil
	}
	return fmt.Errorf("cannot scan %T into CustomDate", src)
}
