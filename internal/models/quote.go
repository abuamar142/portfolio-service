package models

import (
	"time"

	"github.com/google/uuid"
)

type Quote struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Content     string    `json:"content"`
	AuthorName  string    `json:"author_name"`
	IsAnonymous bool      `json:"is_anonymous"`
	Source      string    `json:"source"`
	Color       string    `json:"color"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateQuoteRequest struct {
	Content     string   `json:"content"`
	AuthorName  string   `json:"author_name"`
	IsAnonymous bool     `json:"is_anonymous"`
	Source      string   `json:"source"`
	Tags        []string `json:"tags"`
}

type UpdateQuoteRequest struct {
	Content     string   `json:"content"`
	AuthorName  string   `json:"author_name"`
	IsAnonymous bool     `json:"is_anonymous"`
	Source      string   `json:"source"`
	Tags        []string `json:"tags"`
}

type QuoteListResponse struct {
	Quotes []Quote `json:"quotes"`
	Total  int     `json:"total"`
	Page   int     `json:"page"`
	Limit  int     `json:"limit"`
}

type TagResponse struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type AuthUser struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
}
