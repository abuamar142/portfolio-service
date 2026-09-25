package models

import (
	"time"

	"github.com/google/uuid"
)

type Snippet struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Title       string    `json:"title"`
	Language    string    `json:"language"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateSnippetRequest struct {
	Title       string   `json:"title"`
	Language    string   `json:"language"`
	Code        string   `json:"code"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type UpdateSnippetRequest struct {
	Title       string   `json:"title"`
	Language    string   `json:"language"`
	Code        string   `json:"code"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type SnippetListResponse struct {
	Snippets []Snippet `json:"snippets"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	Limit    int       `json:"limit"`
}

type LanguageResponse struct {
	Language string `json:"language"`
	Count    int    `json:"count"`
}
