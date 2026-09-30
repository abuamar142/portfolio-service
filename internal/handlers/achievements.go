package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/abuamar142/portfolio-service/internal/middleware"
	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/abuamar142/portfolio-service/internal/response"
	"github.com/abuamar142/portfolio-service/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var validAchievementTypes = map[string]bool{
	"certificate":   true,
	"certification": true,
	"webinar":       true,
	"seminar":       true,
	"contribution":  true,
}

// achievementStore is the slice of the service this handler uses.
//
// A narrow interface rather than the concrete *AchievementService so the
// routes can be tested against a fake — the file routes are about the
// ordering of upload, row write and object delete, and the interesting
// failure (the row write failing after a successful upload) cannot be
// provoked against a real database.
type achievementStore interface {
	List(ctx context.Context) (*models.AchievementListResponse, error)
	Create(ctx context.Context, req models.CreateAchievementRequest) (*models.Achievement, error)
	Update(ctx context.Context, id uuid.UUID, req models.UpdateAchievementRequest) (*models.Achievement, error)
	Delete(ctx context.Context, id uuid.UUID) error
	UploadFile(ctx context.Context, token string, id uuid.UUID, contentType string, body []byte, fileName *string, fileSize int64) (*models.Achievement, error)
	DeleteFile(ctx context.Context, token string, id uuid.UUID) (*models.Achievement, error)
}

type AchievementHandler struct {
	AchievementService achievementStore
	OwnerID            string
}

func NewAchievementHandler(svc achievementStore, ownerID string) *AchievementHandler {
	return &AchievementHandler{AchievementService: svc, OwnerID: ownerID}
}

// bearerToken returns the raw token from the Authorization header. The auth
// middleware has already accepted it, so the format is known good.
func bearerToken(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}

// writeFileError maps a file-service failure onto the status the dashboard
// sees.
//
// A rejection from media-service keeps its own status and code, so "the file
// is the wrong type" reads differently from "the media service is down" — the
// first is the user's to fix, the second is a retry.
func writeFileError(w http.ResponseWriter, action string, err error) {
	switch {
	case errors.Is(err, services.ErrMediaUnavailable):
		response.Error(w, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE",
			"file storage is not available, please try again", "")
	default:
		var rejected *services.ErrMediaRejected
		if errors.As(err, &rejected) {
			message := rejected.Message
			if message == "" {
				message = "file rejected"
			}
			response.Error(w, rejected.Status, rejected.Code, message, "")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"failed to "+action+" file", err.Error())
	}
}

func validateAchievementPayload(title, date, typ, organizer, certificateNumber, participantAs string) string {
	if strings.TrimSpace(title) == "" {
		return "title is required"
	}
	if len(title) > 255 {
		return "title too long, max 255 characters"
	}
	if len(date) == 0 {
		return "date is required"
	}
	if _, err := models.ParseCustomDate(date); err != nil {
		return "date must be YYYY-MM-DD"
	}
	if !validAchievementTypes[typ] {
		return "type must be one of: certificate, certification, webinar, seminar, contribution"
	}
	if len(organizer) > 255 {
		return "organizer too long, max 255 characters"
	}
	if len(certificateNumber) > 100 {
		return "certificate_number too long, max 100 characters"
	}
	if len(participantAs) > 160 {
		return "participant_as too long, max 160 characters"
	}
	return ""
}

// ownerOnly gates a handler on a valid token AND OWNER_USER_ID, mirroring the
// links/snippets fail-closed owner checks (empty config denies everyone).
func (h *AchievementHandler) ownerOnly(w http.ResponseWriter, r *http.Request, action string) bool {
	user := middleware.GetUser(r.Context())
	if user == nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated", "")
		return false
	}
	if h.OwnerID == "" || user.ID.String() != h.OwnerID {
		response.Error(w, http.StatusForbidden, "OWNER_ONLY", "only the owner can "+action+" achievements", "")
		return false
	}
	return true
}

// List godoc
// @Summary      List achievements
// @Description  Public list of portfolio achievements (certificates, seminars, webinars)
// @Tags         achievements
// @Produce      json
// @Success      200 {object} models.AchievementListResponse
// @Router       /achievements [get]
func (h *AchievementHandler) List(w http.ResponseWriter, r *http.Request) {
	result, err := h.AchievementService.List(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list achievements", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, "achievements retrieved", result)
}

// Create godoc
// @Summary      Create an achievement (owner only)
// @Description  Add a new achievement to the portfolio
// @Tags         achievements
// @Accept       json
// @Produce      json
// @Param        body body models.CreateAchievementRequest true "Achievement payload"
// @Success      201 {object} models.Achievement
// @Failure      400 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Router       /achievements [post]
func (h *AchievementHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.ownerOnly(w, r, "create") {
		return
	}

	var req models.CreateAchievementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Organizer = strings.TrimSpace(req.Organizer)
	req.Date = strings.TrimSpace(req.Date)
	req.Type = strings.TrimSpace(req.Type)
	req.CertificateNumber = strings.TrimSpace(req.CertificateNumber)
	req.ParticipantAs = strings.TrimSpace(req.ParticipantAs)
	req.Description = strings.TrimSpace(req.Description)

	if msg := validateAchievementPayload(req.Title, req.Date, req.Type, req.Organizer, req.CertificateNumber, req.ParticipantAs); msg != "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", msg, "")
		return
	}

	a, err := h.AchievementService.Create(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create achievement", err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, "achievement created", a)
}

// Update godoc
// @Summary      Update an achievement (owner only)
// @Description  Update an existing achievement
// @Tags         achievements
// @Accept       json
// @Produce      json
// @Param        id path string true "Achievement ID"
// @Param        body body models.UpdateAchievementRequest true "Achievement payload"
// @Success      200 {object} models.Achievement
// @Failure      400 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Failure      404 {object} response.Response
// @Router       /achievements/{id} [put]
func (h *AchievementHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !h.ownerOnly(w, r, "update") {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid achievement ID", "")
		return
	}

	var req models.UpdateAchievementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Organizer = strings.TrimSpace(req.Organizer)
	req.Date = strings.TrimSpace(req.Date)
	req.Type = strings.TrimSpace(req.Type)
	req.CertificateNumber = strings.TrimSpace(req.CertificateNumber)
	req.ParticipantAs = strings.TrimSpace(req.ParticipantAs)
	req.Description = strings.TrimSpace(req.Description)

	if msg := validateAchievementPayload(req.Title, req.Date, req.Type, req.Organizer, req.CertificateNumber, req.ParticipantAs); msg != "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", msg, "")
		return
	}

	a, err := h.AchievementService.Update(r.Context(), id, req)
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "achievement not found", "")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update achievement", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, "achievement updated", a)
}

// Delete godoc
// @Summary      Delete an achievement (owner only)
// @Description  Remove an achievement from the portfolio
// @Tags         achievements
// @Produce      json
// @Param        id path string true "Achievement ID"
// @Success      200 {object} response.Response
// @Failure      400 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Failure      404 {object} response.Response
// @Router       /achievements/{id} [delete]
func (h *AchievementHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.ownerOnly(w, r, "delete") {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid achievement ID", "")
		return
	}

	if err := h.AchievementService.Delete(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "achievement not found", "")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete achievement", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, "achievement deleted", nil)
}

// UploadFile godoc
// @Summary      Upload a certificate file (owner only)
// @Description  Upload a certificate file for an achievement (stored via media-service)
// @Tags         achievements
// @Accept       multipart/form-data
// @Produce      json
// @Param        id path string true "Achievement ID"
// @Param        file formData file true "Certificate file (PDF, PNG, JPEG, WebP; max 10 MB)"
// @Success      200 {object} models.Achievement
// @Failure      400 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Failure      404 {object} response.Response
// @Failure      503 {object} response.Response
// @Router       /achievements/{id}/file [post]
func (h *AchievementHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if !h.ownerOnly(w, r, "upload file") {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid achievement ID", "")
		return
	}

	// Limit body to 10 MB + 1 byte to detect over-limit.
	const maxBytes = 10<<20 + 1
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_CONTENT_TYPE", "Content-Type header is required", "")
		return
	}

	// Extract media type (strip params like charset).
	mediaType := contentType
	if idx := strings.IndexByte(contentType, ';'); idx != -1 {
		mediaType = strings.TrimSpace(contentType[:idx])
	}

	allowed := map[string]bool{
		"application/pdf": true,
		"image/png":       true,
		"image/jpeg":      true,
		"image/webp":      true,
	}
	if !allowed[mediaType] {
		response.Error(w, http.StatusBadRequest, "UNSUPPORTED_MEDIA_TYPE",
			"content type must be one of: application/pdf, image/png, image/jpeg, image/webp", "")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		if err.Error() == "http: request body too large" {
			response.Error(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "file must be 10 MB or smaller", "")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read request body", err.Error())
		return
	}
	if int64(len(body)) > 10<<20 {
		response.Error(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "file must be 10 MB or smaller", "")
		return
	}

	// Read and sanitize X-File-Name header.
	var fileName *string
	if rawName := r.Header.Get("X-File-Name"); rawName != "" {
		decoded, dErr := url.QueryUnescape(rawName)
		if dErr == nil {
			rawName = decoded
		}
		base := filepath.Base(rawName)
		if base == "." || base == "/" {
			base = ""
		}
		// Strip control characters.
		base = strings.Map(func(r rune) rune {
			if r < 32 || r == 127 {
				return -1
			}
			return r
		}, base)
		if len(base) > 255 {
			base = base[:255]
		}
		if base != "" {
			fileName = &base
		}
	}
	fileSize := int64(len(body))

	// The caller's own token is forwarded to media-service, which validates it
	// again. No service credential is minted here, so a token that is revoked
	// cannot upload either.
	a, err := h.AchievementService.UploadFile(r.Context(), bearerToken(r), id, mediaType, body, fileName, fileSize)
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "achievement not found", "")
		return
	}
	if err != nil {
		writeFileError(w, "upload", err)
		return
	}

	response.JSON(w, http.StatusOK, "file uploaded", a)
}

// DeleteFile godoc
// @Summary      Delete a certificate file (owner only)
// @Description  Remove the certificate file from an achievement
// @Tags         achievements
// @Produce      json
// @Param        id path string true "Achievement ID"
// @Success      200 {object} models.Achievement
// @Failure      400 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response
// @Failure      404 {object} response.Response
// @Router       /achievements/{id}/file [delete]
func (h *AchievementHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	if !h.ownerOnly(w, r, "delete file") {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_ID", "invalid achievement ID", "")
		return
	}

	a, err := h.AchievementService.DeleteFile(r.Context(), bearerToken(r), id)
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "achievement not found", "")
		return
	}
	if err != nil {
		writeFileError(w, "delete", err)
		return
	}

	response.JSON(w, http.StatusOK, "file deleted", a)
}
