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

type SnippetHandler struct {
	SnippetService *services.SnippetService
	OwnerID        string
}

func NewSnippetHandler(svc *services.SnippetService, ownerID string) *SnippetHandler {
	return &SnippetHandler{SnippetService: svc, OwnerID: ownerID}
}

// maxSnippetBytes caps stored code size. Code snippets are small; anything
// larger is almost certainly a mistake or abuse of the public share page.
const maxSnippetBytes = 100 * 1024

func validateSnippetPayload(title, language, code string, tags []string) string {
	if strings.TrimSpace(title) == "" {
		return "title is required"
	}
	if len(title) > 255 {
		return "title must be at most 255 characters"
	}
	if strings.TrimSpace(language) == "" {
		return "language is required"
	}
	if len(language) > 50 {
		return "language must be at most 50 characters"
	}
	if strings.TrimSpace(code) == "" {
		return "code is required"
	}
	if len(code) > maxSnippetBytes {
		return "code must be at most 100KB"
	}
	if len(tags) > 20 {
		return "at most 20 tags are allowed"
	}
	for _, t := range tags {
		if len(t) > 50 {
			return "each tag must be at most 50 characters"
		}
	}
	return ""
}

// List godoc
// @Summary      List snippets
// @Description  Get all snippets with optional search, tag and language filter
// @Tags         snippets
// @Produce      json
// @Param        search query string false "Search in title, description or code"
// @Param        tag query string false "Filter by tag"
// @Param        language query string false "Filter by language"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(20)
// @Success      200 {object} models.SnippetListResponse
// @Router       /snippets [get]
func (h *SnippetHandler) List(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	tags := splitTags(r.URL.Query().Get("tag"))
	language := r.URL.Query().Get("language")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	result, err := h.SnippetService.List(r.Context(), search, tags, language, page, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list snippets", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, "snippets retrieved", result)
}

// GetByID godoc
// @Summary      Get snippet by ID
// @Description  Get a single snippet
// @Tags         snippets
// @Produce      json
// @Param        id path string true "Snippet ID"
// @Success      200 {object} models.Snippet
// @Router       /snippets/{id} [get]
func (h *SnippetHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid snippet ID", "")
		return
	}

	snippet, err := h.SnippetService.GetByID(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "snippet not found", "")
		return
	}

	response.JSON(w, http.StatusOK, "snippet retrieved", snippet)
}

// ListTags godoc
// @Summary      List snippet tags
// @Description  Get all tags used by snippets, with counts
// @Tags         snippets
// @Produce      json
// @Success      200 {array} models.TagResponse
// @Router       /snippets/tags [get]
func (h *SnippetHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.SnippetService.ListTags(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list tags", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, "tags retrieved", tags)
}

// ListLanguages godoc
// @Summary      List snippet languages
// @Description  Get distinct languages in use, with counts
// @Tags         snippets
// @Produce      json
// @Success      200 {array} models.LanguageResponse
// @Router       /snippets/languages [get]
func (h *SnippetHandler) ListLanguages(w http.ResponseWriter, r *http.Request) {
	langs, err := h.SnippetService.ListLanguages(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list languages", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, "languages retrieved", langs)
}

// Create godoc
// @Summary      Create a snippet
// @Description  Create a new snippet (owner only)
// @Tags         snippets
// @Accept       json
// @Produce      json
// @Param        body body models.CreateSnippetRequest true "Snippet payload"
// @Success      201 {object} models.Snippet
// @Router       /snippets [post]
func (h *SnippetHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated", "")
		return
	}

	// Owner-curated, like links: fails closed unless caller matches OWNER_USER_ID.
	if h.OwnerID == "" || user.ID.String() != h.OwnerID {
		response.Error(w, http.StatusForbidden, "OWNER_ONLY", "only the owner can create snippets", "")
		return
	}

	var req models.CreateSnippetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	if msg := validateSnippetPayload(req.Title, req.Language, req.Code, req.Tags); msg != "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", msg, "")
		return
	}

	snippet, err := h.SnippetService.Create(r.Context(), user.ID, req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create snippet", err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, "snippet created", snippet)
}

// Update godoc
// @Summary      Update a snippet
// @Description  Update an existing snippet (owner only)
// @Tags         snippets
// @Accept       json
// @Produce      json
// @Param        id path string true "Snippet ID"
// @Param        body body models.UpdateSnippetRequest true "Snippet payload"
// @Success      200 {object} models.Snippet
// @Router       /snippets/{id} [put]
func (h *SnippetHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated", "")
		return
	}

	snippetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid snippet ID", "")
		return
	}

	var req models.UpdateSnippetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	if msg := validateSnippetPayload(req.Title, req.Language, req.Code, req.Tags); msg != "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", msg, "")
		return
	}

	snippet, err := h.SnippetService.Update(r.Context(), user.ID, snippetID, req)
	if err != nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "snippet not found or not owned by user", "")
		return
	}

	response.JSON(w, http.StatusOK, "snippet updated", snippet)
}

// Delete godoc
// @Summary      Delete a snippet
// @Description  Delete an existing snippet (owner only)
// @Tags         snippets
// @Produce      json
// @Param        id path string true "Snippet ID"
// @Success      200 {object} response.Response
// @Router       /snippets/{id} [delete]
func (h *SnippetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated", "")
		return
	}

	snippetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid snippet ID", "")
		return
	}

	if err := h.SnippetService.Delete(r.Context(), user.ID, snippetID); err != nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "snippet not found or not owned by user", "")
		return
	}

	response.JSON(w, http.StatusOK, "snippet deleted", nil)
}
