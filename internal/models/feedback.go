package models

import (
	"time"

	"github.com/google/uuid"
)

// Feedback is a short anonymous message left by a visitor. There is no
// user_id: submissions are public and unauthenticated by design.
type Feedback struct {
	ID        uuid.UUID `json:"id"`
	Message   string    `json:"message"`
	Contact   string    `json:"contact"`
	PageURL   string    `json:"page_url"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateFeedbackRequest struct {
	Message string `json:"message"`
	Contact string `json:"contact"`
	PageURL string `json:"page_url"`
	// Website is a honeypot: humans never see or fill it. A non-empty value
	// means a bot submitted the form — the API answers with success but
	// stores nothing.
	Website string `json:"website"`
}

type UpdateFeedbackRequest struct {
	Status string `json:"status"`
}

type FeedbackListResponse struct {
	Feedback []Feedback `json:"feedback"`
	Total    int        `json:"total"`
}
