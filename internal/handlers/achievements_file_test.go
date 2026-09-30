package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/abuamar142/portfolio-service/internal/middleware"
	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/abuamar142/portfolio-service/internal/services"
)

// The routes and the envelope the dashboard already parses:
//
//	uploadAchievementFile() -> POST /achievements/{id}/file, reads data.data
//	deleteAchievementFile() -> DELETE /achievements/{id}/file, reads data.data
//
// So `data` must stay the achievement object itself and the status must stay
// 200. These tests exercise the real handler through the real middleware, with
// a stubbed auth-service and a stubbed media-service.

const testOwnerID = "11111111-1111-1111-1111-111111111111"

// stubAuthService accepts one token and returns an AuthUser with the owner id
// the handler checks against.
func stubAuthService(t *testing.T, ownerID string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/me" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{"success": false})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"id": ownerID, "email": "owner@example.test"},
		})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// stubFileMedia is a stand-in for media-service at the HTTP level.
type stubFileMedia struct {
	*httptest.Server
	uploads   []string
	auths     []string
	deletes   []string
	upStatus  int
	upCode    string
	upMessage string
}

func newStubFileMedia(t *testing.T) *stubFileMedia {
	t.Helper()
	s := &stubFileMedia{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/upload"):
			s.uploads = append(s.uploads, r.URL.RawQuery)
			s.auths = append(s.auths, r.Header.Get("Authorization"))
			if s.upStatus != 0 {
				w.WriteHeader(s.upStatus)
				json.NewEncoder(w).Encode(map[string]any{
					"success": false, "message": s.upMessage,
					"error": map[string]string{"code": s.upCode},
				})
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "message": "media uploaded",
				"data": map[string]string{
					"url": "https://files.abuamar.online/certificates/abc/file-1.pdf",
					"key": "certificates/abc/file-1.pdf",
				},
			})
		case strings.HasSuffix(r.URL.Path, "/object"):
			var req struct {
				URL string `json:"url"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			s.deletes = append(s.deletes, req.URL)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "media deleted"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.Server.Close)
	return s
}

// fakeAchievementStore stands in for the service. The file routes are what
// these tests cover, so the CRUD methods are stubs; the file methods mirror
// the real service's shape (including storing the KEY, not the URL) so the
// response envelope is exercised end to end.
type fakeAchievementStore struct {
	mediaURL string
	current  string
	key      string
}

func (f *fakeAchievementStore) List(ctx context.Context) (*models.AchievementListResponse, error) {
	return &models.AchievementListResponse{}, nil
}

func (f *fakeAchievementStore) Create(ctx context.Context, req models.CreateAchievementRequest) (*models.Achievement, error) {
	return &models.Achievement{}, nil
}

func (f *fakeAchievementStore) Update(ctx context.Context, id uuid.UUID, req models.UpdateAchievementRequest) (*models.Achievement, error) {
	return &models.Achievement{ID: id}, nil
}

func (f *fakeAchievementStore) Delete(ctx context.Context, id uuid.UUID) error { return nil }

func (f *fakeAchievementStore) UploadFile(ctx context.Context, token string, id uuid.UUID, contentType string, body []byte, fileName *string, fileSize int64) (*models.Achievement, error) {
	// A real client, so the request that reaches the stub is the one the
	// production code would send (URL, token, query params).
	client := services.NewMediaClient(f.mediaURL)
	_, key, err := client.Upload(ctx, token, "certificates", id.String(), "file", body)
	if err != nil {
		return nil, err
	}
	// The row now holds the key, so a later delete finds it — mirroring the
	// real service, where the persisted key is what cleanup reads.
	f.key = key
	f.current = key
	return &models.Achievement{ID: id, FileKey: key}, nil
}

func (f *fakeAchievementStore) DeleteFile(ctx context.Context, token string, id uuid.UUID) (*models.Achievement, error) {
	client := services.NewMediaClient(f.mediaURL)
	if f.current != "" {
		if err := client.Delete(ctx, token, f.current); err != nil {
			return &models.Achievement{ID: id}, err
		}
	}
	f.current = ""
	return &models.Achievement{ID: id, FileKey: ""}, nil
}

func TestAchievementFileRoutes(t *testing.T) {
	media := newStubFileMedia(t)
	authURL := stubAuthService(t, testOwnerID)

	h := NewAchievementHandler(&fakeAchievementStore{mediaURL: media.URL}, testOwnerID)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(authURL))
			r.Post("/achievements/{id}/file", h.UploadFile)
			r.Delete("/achievements/{id}/file", h.DeleteFile)
		})
	})

	achID := "22222222-2222-2222-2222-222222222222"
	pdf := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte{0}, 64)...)

	t.Run("upload keeps the envelope the dashboard reads", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/achievements/"+achID+"/file", bytes.NewReader(pdf))
		req.Header.Set("Authorization", "Bearer good-token")
		req.Header.Set("Content-Type", "application/pdf")
		req.Header.Set("X-File-Name", "certificate.pdf")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		var env map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("response is not JSON: %v", err)
		}
		if env["success"] != true {
			t.Errorf("success = %v, want true", env["success"])
		}
		// data must be the achievement object, not a wrapper.
		data, ok := env["data"].(map[string]any)
		if !ok {
			t.Fatalf("data = %T, want the achievement object", env["data"])
		}
		if _, wrapped := data["item"]; wrapped {
			t.Error("data is wrapped in {item: ...}; the dashboard reads data as the achievement")
		}
		key, _ := data["file_key"].(string)
		if key == "" || strings.HasPrefix(key, "https://") {
			t.Errorf("data.file_key = %q, want a bare object key", key)
		}
		if len(media.uploads) != 1 {
			t.Fatalf("uploads = %v, want 1", media.uploads)
		}
		if !strings.Contains(media.uploads[0], "resource=certificates") {
			t.Errorf("query = %q, want resource=certificates", media.uploads[0])
		}
		if len(media.auths) != 1 || media.auths[0] != "Bearer good-token" {
			t.Errorf("forwarded %v, want the caller's token", media.auths)
		}
	})

	t.Run("a PDF is accepted, not rejected as an unsupported type", func(t *testing.T) {
		// The point of extending media-service: a certificate is a PDF.
		if len(media.uploads) == 0 {
			t.Fatal("no upload reached media-service")
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/achievements/"+achID+"/file", bytes.NewReader(pdf))
		req.Header.Set("Authorization", "Bearer good-token")
		req.Header.Set("Content-Type", "application/pdf")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusBadRequest {
			t.Errorf("a PDF was rejected with 400: %s", rec.Body.String())
		}
	})

	t.Run("media-service down is a clean 503", func(t *testing.T) {
		media.upStatus = http.StatusInternalServerError
		defer func() { media.upStatus = 0 }()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/achievements/"+achID+"/file", bytes.NewReader(pdf))
		req.Header.Set("Authorization", "Bearer good-token")
		req.Header.Set("Content-Type", "application/pdf")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
		}
		var env map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		errBody, _ := env["error"].(map[string]any)
		if errBody["code"] != "STORAGE_UNAVAILABLE" {
			t.Errorf("error.code = %v, want STORAGE_UNAVAILABLE", errBody["code"])
		}
	})

	t.Run("media rejection keeps its own status and code", func(t *testing.T) {
		media.upStatus = http.StatusUnsupportedMediaType
		media.upCode = "UNSUPPORTED_TYPE"
		media.upMessage = "format must be JPEG, PNG, WebP, AVIF, SVG or PDF"
		defer func() {
			media.upStatus, media.upCode, media.upMessage = 0, "", ""
		}()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/achievements/"+achID+"/file", bytes.NewReader(pdf))
		req.Header.Set("Authorization", "Bearer good-token")
		req.Header.Set("Content-Type", "application/pdf")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("status = %d, want 415", rec.Code)
		}
	})

	t.Run("delete keeps the envelope and removes the object", func(t *testing.T) {
		before := len(media.deletes)
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/achievements/"+achID+"/file", nil)
		req.Header.Set("Authorization", "Bearer good-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		var env map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		if _, ok := env["data"].(map[string]any); !ok {
			t.Errorf("data = %T, want the achievement object", env["data"])
		}
		if len(media.deletes) != before+1 {
			t.Errorf("deletes = %v, want one more", media.deletes)
		}
	})

	t.Run("unauthenticated is rejected before any upload", func(t *testing.T) {
		before := len(media.uploads)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/achievements/"+achID+"/file", bytes.NewReader(pdf))
		req.Header.Set("Content-Type", "application/pdf")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
		if len(media.uploads) != before {
			t.Error("an unauthenticated request reached media-service")
		}
	})
}
