package handlers

import (
	"net/http"

	"github.com/abuamar142/portfolio-service/internal/response"
	"github.com/abuamar142/portfolio-service/internal/services"
)

// ProfileHandler serves the identity block and the four ordered lists that
// replaced the Payload CMS. Public and read-only: this is the data the homepage
// shows every visitor, and nothing here is owner-gated.
type ProfileHandler struct {
	ProfileService *services.ProfileService
}

func NewProfileHandler(svc *services.ProfileService) *ProfileHandler {
	return &ProfileHandler{ProfileService: svc}
}

// Get godoc
// @Summary      Get portfolio profile
// @Description  Identity block plus skills, experiences, projects and education
// @Tags         profile
// @Produce      json
// @Success      200 {object} models.ProfileResponse
// @Failure      500 {object} response.Response
// @Router       /profile [get]
func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	result, err := h.ProfileService.Get(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"failed to load profile", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, "profile retrieved", result)
}
