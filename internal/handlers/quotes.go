package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/abuamar142/portfolio-service/internal/middleware"
	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/abuamar142/portfolio-service/internal/response"
	"github.com/abuamar142/portfolio-service/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type QuoteHandler struct {
	QuoteService *services.QuoteService
}

func NewQuoteHandler(svc *services.QuoteService) *QuoteHandler {
	return &QuoteHandler{QuoteService: svc}
}

// List godoc
// @Summary      List quotes
// @Description  Get all quotes with optional search and tag filter
// @Tags         quotes
// @Produce      json
// @Param        search query string false "Search in content"
// @Param        tag query string false "Filter by tag"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(20)
// @Success      200 {object} models.QuoteListResponse
// @Router       /quotes [get]
func (h *QuoteHandler) List(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	tags := splitTags(r.URL.Query().Get("tag"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	result, err := h.QuoteService.List(r.Context(), search, tags, page, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list quotes", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, "quotes retrieved", result)
}

// GetByID godoc
// @Summary      Get quote by ID
// @Description  Get a single quote
// @Tags         quotes
// @Produce      json
// @Param        id path string true "Quote ID"
// @Success      200 {object} models.Quote
// @Router       /quotes/{id} [get]
func (h *QuoteHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid quote ID", "")
		return
	}

	quote, err := h.QuoteService.GetByID(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "quote not found", "")
		return
	}

	response.JSON(w, http.StatusOK, "quote retrieved", quote)
}

// Create godoc
// @Summary      Create a quote
// @Description  Create a new quote (requires authentication)
// @Tags         quotes
// @Accept       json
// @Produce      json
// @Param        body body models.CreateQuoteRequest true "Quote payload"
// @Success      201 {object} models.Quote
// @Failure      400 {object} response.Response
// @Failure      401 {object} response.Response
// @Router       /quotes [post]
func (h *QuoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated", "")
		return
	}

	var req models.CreateQuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	if req.Content == "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "content is required", "")
		return
	}
	if len(req.Content) > 500 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "content too long", "max 500 characters")
		return
	}
	if len(req.Tags) > 5 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "too many tags", "max 5 tags")
		return
	}

	quote, err := h.QuoteService.Create(r.Context(), user.ID, req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create quote", err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, "quote created", quote)
}

// Update godoc
// @Summary      Update a quote
// @Description  Update an existing quote (owner only)
// @Tags         quotes
// @Accept       json
// @Produce      json
// @Param        id path string true "Quote ID"
// @Param        body body models.UpdateQuoteRequest true "Quote payload"
// @Success      200 {object} models.Quote
// @Failure      400 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      404 {object} response.Response
// @Router       /quotes/{id} [put]
func (h *QuoteHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated", "")
		return
	}

	quoteID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid quote ID", "")
		return
	}

	var req models.UpdateQuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	if req.Content == "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "content is required", "")
		return
	}
	if len(req.Content) > 500 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "content too long", "max 500 characters")
		return
	}

	quote, err := h.QuoteService.Update(r.Context(), user.ID, quoteID, req)
	if err != nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "quote not found or not owned by user", "")
		return
	}

	response.JSON(w, http.StatusOK, "quote updated", quote)
}

// Delete godoc
// @Summary      Delete a quote
// @Description  Delete a quote (owner only)
// @Tags         quotes
// @Produce      json
// @Param        id path string true "Quote ID"
// @Success      200 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      404 {object} response.Response
// @Router       /quotes/{id} [delete]
func (h *QuoteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated", "")
		return
	}

	quoteID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid quote ID", "")
		return
	}

	if err := h.QuoteService.Delete(r.Context(), user.ID, quoteID); err != nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "quote not found or not owned by user", "")
		return
	}

	response.JSON(w, http.StatusOK, "quote deleted", nil)
}

// ListTags godoc
// @Summary      List all tags
// @Description  Get all tags with their quote count
// @Tags         quotes
// @Produce      json
// @Success      200 {array} models.TagResponse
// @Router       /quotes/tags [get]
func (h *QuoteHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.QuoteService.ListTags(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list tags", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, "tags retrieved", tags)
}
