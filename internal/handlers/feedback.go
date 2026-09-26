package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/abuamar142/portfolio-service/internal/middleware"
	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/abuamar142/portfolio-service/internal/response"
	"github.com/abuamar142/portfolio-service/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type FeedbackHandler struct {
	FeedbackService *services.FeedbackService
	OwnerID         string
	// Notify sends best-effort notifications (Telegram). Optional: nil is a
	// valid no-op, and callers invoke it from their own goroutine.
	Notify func(text string)
}

func NewFeedbackHandler(svc *services.FeedbackService, ownerID string, notify func(string)) *FeedbackHandler {
	return &FeedbackHandler{FeedbackService: svc, OwnerID: ownerID, Notify: notify}
}

func validateFeedbackPayload(message, contact, pageURL string) string {
	if strings.TrimSpace(message) == "" {
		return "message is required"
	}
	if len(message) > 500 {
		return "message too long, max 500 characters"
	}
	if len(contact) > 120 {
		return "contact too long, max 120 characters"
	}
	if pageURL != "" && !strings.HasPrefix(pageURL, "http://") && !strings.HasPrefix(pageURL, "https://") {
		return "page_url must start with http:// or https://"
	}
	if len(pageURL) > 300 {
		return "page_url too long, max 300 characters"
	}
	return ""
}

// ownerOnly gates a handler on a valid token AND OWNER_USER_ID, mirroring the
// links/snippets fail-closed owner checks (empty config denies everyone).
func (h *FeedbackHandler) ownerOnly(w http.ResponseWriter, r *http.Request, action string) bool {
	user := middleware.GetUser(r.Context())
	if user == nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated", "")
		return false
	}
	if h.OwnerID == "" || user.ID.String() != h.OwnerID {
		response.Error(w, http.StatusForbidden, "OWNER_ONLY", "only the owner can "+action+" feedback", "")
		return false
	}
	return true
}

// Create godoc
// @Summary      Submit feedback
// @Description  Public, anonymous: a visitor leaves a short message for the owner. A filled honeypot field is accepted but discarded.
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Param        body body models.CreateFeedbackRequest true "Feedback payload"
// @Success      201 {object} models.Feedback
// @Failure      400 {object} response.Response
// @Router       /feedback [post]
func (h *FeedbackHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	req.Contact = strings.TrimSpace(req.Contact)
	req.PageURL = strings.TrimSpace(req.PageURL)

	// Honeypot tripped: answer exactly like a real submission so bots cannot
	// tell the difference, but store nothing and notify nobody.
	if strings.TrimSpace(req.Website) != "" {
		response.JSON(w, http.StatusCreated, "feedback stored", map[string]string{"id": uuid.New().String()})
		return
	}

	if msg := validateFeedbackPayload(req.Message, req.Contact, req.PageURL); msg != "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", msg, "")
		return
	}

	f, err := h.FeedbackService.Create(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to store feedback", err.Error())
		return
	}

	if h.Notify != nil {
		// Fire-and-forget: the Telegram round-trip must never slow the POST.
		// User text goes through html.EscapeString because the notifier uses
		// Telegram's HTML parse mode.
		text := fmt.Sprintf("📨 <b>Feedback baru</b>\n💬 %s", html.EscapeString(req.Message))
		if req.Contact != "" {
			text += fmt.Sprintf("\n📮 %s", html.EscapeString(req.Contact))
		}
		if req.PageURL != "" {
			text += fmt.Sprintf("\n🌐 %s", html.EscapeString(req.PageURL))
		}
		go h.Notify(text)
	}

	response.JSON(w, http.StatusCreated, "feedback stored", f)
}

// List godoc
// @Summary      List feedback (owner only)
// @Description  Newest-first inbox, optionally filtered by status
// @Tags         feedback
// @Produce      json
// @Param        status query string false "Filter: new, read or all" Enums(new, read, all)
// @Success      200 {object} models.FeedbackListResponse
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Router       /feedback [get]
func (h *FeedbackHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.ownerOnly(w, r, "read") {
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "all"
	}
	if status != "all" && status != "new" && status != "read" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be one of: all, new, read", "")
		return
	}
	if status == "all" {
		status = ""
	}

	result, err := h.FeedbackService.List(r.Context(), status)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list feedback", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, "feedback retrieved", result)
}

// Update godoc
// @Summary      Update feedback status (owner only)
// @Description  Toggle an item between "new" and "read"
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Param        id path string true "Feedback ID"
// @Param        body body models.UpdateFeedbackRequest true "Status payload"
// @Success      200 {object} models.Feedback
// @Failure      400 {object} response.Response
// @Failure      404 {object} response.Response
// @Router       /feedback/{id} [patch]
func (h *FeedbackHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !h.ownerOnly(w, r, "update") {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid feedback ID", "")
		return
	}

	var req models.UpdateFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}
	if req.Status != "new" && req.Status != "read" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be one of: new, read", "")
		return
	}

	f, err := h.FeedbackService.UpdateStatus(r.Context(), id, req.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "feedback not found", "")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update feedback", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, "feedback updated", f)
}

// Delete godoc
// @Summary      Delete feedback (owner only)
// @Description  Remove a message from the inbox
// @Tags         feedback
// @Produce      json
// @Param        id path string true "Feedback ID"
// @Success      200 {object} response.Response
// @Failure      400 {object} response.Response
// @Failure      404 {object} response.Response
// @Router       /feedback/{id} [delete]
func (h *FeedbackHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.ownerOnly(w, r, "delete") {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid feedback ID", "")
		return
	}

	if err := h.FeedbackService.Delete(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "feedback not found", "")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete feedback", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, "feedback deleted", nil)
}
