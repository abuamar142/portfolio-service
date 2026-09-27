package models

import (
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
