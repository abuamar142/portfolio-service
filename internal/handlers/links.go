package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/abuamar142/portfolio-service/internal/middleware"
	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/abuamar142/portfolio-service/internal/response"
	"github.com/abuamar142/portfolio-service/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type LinkHandler struct {
	LinkService *services.LinkService
}

func NewLinkHandler(svc *services.LinkService) *LinkHandler {
	return &LinkHandler{LinkService: svc}
}

func validateLinkPayload(url, title, description string, tags []string) string {
	if url == "" {
		return "url is required"
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "url must start with http:// or https://"
	}
	if len(url) > 2048 {
		return "url too long, max 2048 characters"
	}
	if title == "" {
		return "title is required"
	}
	if len(title) > 255 {
		return "title too long, max 255 characters"
	}
	if len(description) > 500 {
		return "description too long, max 500 characters"
	}
	if len(tags) > 5 {
		return "too many tags, max 5"
	}
	return ""
}

// List godoc
// @Summary      List links
// @Description  Get all links with optional search and tag filter
// @Tags         links
// @Produce      json
// @Param        search query string false "Search in title, description or url"
// @Param        tag query string false "Filter by tag"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(20)
// @Success      200 {object} models.LinkListResponse
// @Router       /links [get]
func (h *LinkHandler) List(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	tag := r.URL.Query().Get("tag")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	result, err := h.LinkService.List(r.Context(), search, tag, page, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list links", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, "links retrieved", result)
}

// GetByID godoc
// @Summary      Get link by ID
// @Description  Get a single link
// @Tags         links
// @Produce      json
// @Param        id path string true "Link ID"
// @Success      200 {object} models.Link
// @Router       /links/{id} [get]
func (h *LinkHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid link ID", "")
		return
	}

	link, err := h.LinkService.GetByID(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "link not found", "")
		return
	}

	response.JSON(w, http.StatusOK, "link retrieved", link)
}

// Create godoc
// @Summary      Create a link
// @Description  Create a new link (requires authentication)
// @Tags         links
// @Accept       json
// @Produce      json
// @Param        body body models.CreateLinkRequest true "Link payload"
// @Success      201 {object} models.Link
// @Failure      400 {object} response.Response
// @Failure      401 {object} response.Response
// @Router       /links [post]
func (h *LinkHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated", "")
		return
	}

	var req models.CreateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	if msg := validateLinkPayload(req.URL, req.Title, req.Description, req.Tags); msg != "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", msg, "")
		return
	}

	link, err := h.LinkService.Create(r.Context(), user.ID, req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create link", err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, "link created", link)
}

// Update godoc
// @Summary      Update a link
// @Description  Update an existing link (owner only)
// @Tags         links
// @Accept       json
// @Produce      json
// @Param        id path string true "Link ID"
// @Param        body body models.UpdateLinkRequest true "Link payload"
// @Success      200 {object} models.Link
// @Failure      400 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      404 {object} response.Response
// @Router       /links/{id} [put]
func (h *LinkHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated", "")
		return
	}

	linkID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid link ID", "")
		return
	}

	var req models.UpdateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	if msg := validateLinkPayload(req.URL, req.Title, req.Description, req.Tags); msg != "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", msg, "")
		return
	}

	link, err := h.LinkService.Update(r.Context(), user.ID, linkID, req)
	if err != nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "link not found or not owned by user", "")
		return
	}

	response.JSON(w, http.StatusOK, "link updated", link)
}

// Delete godoc
// @Summary      Delete a link
// @Description  Delete a link (owner only)
// @Tags         links
// @Produce      json
// @Param        id path string true "Link ID"
// @Success      200 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      404 {object} response.Response
// @Router       /links/{id} [delete]
func (h *LinkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated", "")
		return
	}

	linkID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid link ID", "")
		return
	}

	if err := h.LinkService.Delete(r.Context(), user.ID, linkID); err != nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "link not found or not owned by user", "")
		return
	}

	response.JSON(w, http.StatusOK, "link deleted", nil)
}

// ListTags godoc
// @Summary      List link tags
// @Description  Get all tags used by links with their count
// @Tags         links
// @Produce      json
// @Success      200 {array} models.TagResponse
// @Router       /links/tags [get]
func (h *LinkHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.LinkService.ListTags(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list tags", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, "tags retrieved", tags)
}
