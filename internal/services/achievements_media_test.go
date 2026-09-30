package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/google/uuid"
)

// ── stub media-service ────────────────────────────────────────────────────

type mediaCall struct {
	Method        string
	Project       string
	Resource      string
	ID            string
	Field         string
	Body          []byte
	Authorization string
	DeleteURL     string
}

// stubMedia stands in for media-service, recording what it was asked to do.
//
// The contract this service has to honour is about the request it sends — the
// project it names, the token it forwards, the resource/id/field it passes,
// and whether it asks for the replaced object to be deleted — and none of that
// is visible from the response alone.
type stubMedia struct {
	*httptest.Server

	mu    sync.Mutex
	calls []mediaCall

	uploadStatus int
	uploadBody   string
	deleteStatus int
}

func newStubMedia(t *testing.T) *stubMedia {
	t.Helper()

	s := &stubMedia{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/upload") && r.Method == http.MethodPost:
			body, _ := io.ReadAll(io.LimitReader(r.Body, 16<<20))
			s.mu.Lock()
			n := len(s.calls) + 1
			s.calls = append(s.calls, mediaCall{
				Method:        r.Method,
				Project:       projectOf(r.URL.Path),
				Resource:      r.URL.Query().Get("resource"),
				ID:            r.URL.Query().Get("id"),
				Field:         r.URL.Query().Get("field"),
				Body:          body,
				Authorization: r.Header.Get("Authorization"),
			})
			status, bodyText := s.uploadStatus, s.uploadBody
			s.mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			if status != 0 {
				w.WriteHeader(status)
				if bodyText != "" {
					io.WriteString(w, bodyText)
				}
				return
			}
			w.WriteHeader(http.StatusCreated)
			// A distinct key per upload so a test can tell the new object from
			// the one it replaced.
			key := "certificates/id/file-" + itoa(n) + ".pdf"
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "message": "media uploaded",
				"data": map[string]string{
					"url": "https://files.abuamar.online/" + key,
					"key": key,
				},
			})

		case strings.HasSuffix(r.URL.Path, "/object") && r.Method == http.MethodDelete:
			var req struct {
				URL string `json:"url"`
			}
			_ = json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req)
			s.mu.Lock()
			s.calls = append(s.calls, mediaCall{
				Method:        r.Method,
				Project:       projectOf(r.URL.Path),
				DeleteURL:     req.URL,
				Authorization: r.Header.Get("Authorization"),
			})
			status := s.deleteStatus
			s.mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			if status != 0 {
				w.WriteHeader(status)
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "delete failed"})
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "media deleted"})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.Server.Close)
	return s
}

func (s *stubMedia) callsFor() []mediaCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]mediaCall, len(s.calls))
	copy(out, s.calls)
	return out
}

// deletedURLs returns the URLs it was asked to delete, waiting briefly for the
// detached cleanup goroutine to land.
func (s *stubMedia) deletedURLs(t *testing.T) []string {
	t.Helper()
	var out []string
	for range 50 {
		out = out[:0]
		for _, c := range s.callsFor() {
			if c.Method == http.MethodDelete {
				out = append(out, c.DeleteURL)
			}
		}
		if len(out) > 0 {
			return out
		}
		// The replace path deletes in a goroutine; give it a moment.
		waitABit()
	}
	return out
}

func (s *stubMedia) failUploads(status int, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uploadStatus, s.uploadBody = status, body
}

func (s *stubMedia) failDeletes(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteStatus = status
}

func projectOf(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// api, v1, media, {project}, upload|object
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// ── fake row store ────────────────────────────────────────────────────────

type fakeStore struct {
	current string
	setErr  error
	setKey  string
	calls   []string
}

func (f *fakeStore) currentFileKey(ctx context.Context, id uuid.UUID) (string, error) {
	f.calls = append(f.calls, "resolve")
	return f.current, nil
}

func (f *fakeStore) setFileMetadata(ctx context.Context, id uuid.UUID, key string, fileName *string, fileSize int64) (*models.Achievement, error) {
	f.calls = append(f.calls, "persist")
	if f.setErr != nil {
		return nil, f.setErr
	}
	f.setKey = key
	return &models.Achievement{ID: id, FileKey: key}, nil
}

// newService wires an AchievementService to a fake store and a stub media.
func newService(t *testing.T, store *fakeStore) (*AchievementService, *stubMedia) {
	t.Helper()
	media := newStubMedia(t)
	svc := &AchievementService{media: NewMediaClient(media.URL), store: store}
	return svc, media
}

// waitABit gives the detached replace-cleanup goroutine a chance to land
// before the test reads what media-service was asked to delete.
func waitABit() { time.Sleep(10 * time.Millisecond) }

// A real PDF header. media-service sniffs the bytes, so the fixture has to
// look like what it claims to be.
var pdfBytes = append([]byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n"), make([]byte, 128)...)

var testID = uuid.MustParse("11111111-2222-3333-4444-555555555555")

// ── the upload path ───────────────────────────────────────────────────────

// The happy path: the bytes reach media-service under project "portfolio" with
// the caller's token, and the key it returns is written to the row.
func TestUploadFileSendsToMediaServiceAndPersists(t *testing.T) {
	store := &fakeStore{}
	svc, media := newService(t, store)

	got, err := svc.UploadFile(context.Background(), "the-token", testID, "application/pdf", pdfBytes, nil, int64(len(pdfBytes)))
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}

	calls := media.callsFor()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].Project != "portfolio" {
		t.Errorf("project = %q, want portfolio", calls[0].Project)
	}
	if calls[0].Resource != "certificates" || calls[0].ID != testID.String() || calls[0].Field != "file" {
		t.Errorf("upload = %+v, want certificates/%s/file", calls[0], testID)
	}
	if calls[0].Authorization != "Bearer the-token" {
		t.Errorf("Authorization = %q, want the caller's own token", calls[0].Authorization)
	}
	if string(calls[0].Body) != string(pdfBytes) {
		t.Errorf("body = %d bytes, want %d", len(calls[0].Body), len(pdfBytes))
	}

	// The KEY is stored, not the URL: the dashboard builds the link itself.
	if store.setKey == "" || strings.HasPrefix(store.setKey, "https://") {
		t.Errorf("stored %q, want an object key (the dashboard prefixes it)", store.setKey)
	}
	if got.FileKey != store.setKey {
		t.Errorf("returned key %q, stored %q", got.FileKey, store.setKey)
	}
}

// The row must be resolved before the upload: a rejected request must not
// leave an orphan in the bucket.
func TestUploadFileResolvesBeforeUploading(t *testing.T) {
	store := &fakeStore{}
	svc, media := newService(t, store)

	if _, err := svc.UploadFile(context.Background(), "t", testID, "application/pdf", pdfBytes, nil, int64(len(pdfBytes))); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}

	if len(store.calls) != 2 || store.calls[0] != "resolve" || store.calls[1] != "persist" {
		t.Errorf("order = %v, want [resolve persist]", store.calls)
	}
	// The replaced object is removed after the row is written.
	if deleted := media.deletedURLs(t); len(deleted) != 0 {
		t.Errorf("deleted = %v, want none (there was nothing to replace)", deleted)
	}
}

// Replacing a certificate removes the object it replaced, so the bucket does
// not accumulate one file per re-upload.
func TestUploadFileRemovesTheReplacedObject(t *testing.T) {
	store := &fakeStore{current: "certificates/old-key.pdf"}
	svc, media := newService(t, store)

	if _, err := svc.UploadFile(context.Background(), "t", testID, "application/pdf", pdfBytes, nil, int64(len(pdfBytes))); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}

	deleted := media.deletedURLs(t)
	if len(deleted) != 1 {
		t.Fatalf("deleted = %v, want the replaced object", deleted)
	}
	// A legacy key is stored bare, so the cleanup must accept it — this is the
	// shape every certificate in production has today.
	if deleted[0] != "certificates/old-key.pdf" {
		t.Errorf("deleted %q, want certificates/old-key.pdf", deleted[0])
	}
}

// A first upload has nothing to replace, so no delete is sent.
func TestUploadFileFirstUploadDeletesNothing(t *testing.T) {
	store := &fakeStore{}
	svc, media := newService(t, store)

	if _, err := svc.UploadFile(context.Background(), "t", testID, "application/pdf", pdfBytes, nil, int64(len(pdfBytes))); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if deleted := media.deletedURLs(t); len(deleted) != 0 {
		t.Errorf("deleted = %v, want none", deleted)
	}
}

// If the row write fails after a successful upload, the new object is removed
// rather than left as an orphan.
func TestUploadFileRemovesNewObjectWhenPersistFails(t *testing.T) {
	store := &fakeStore{setErr: errors.New("db down")}
	svc, media := newService(t, store)

	if _, err := svc.UploadFile(context.Background(), "t", testID, "application/pdf", pdfBytes, nil, int64(len(pdfBytes))); err == nil {
		t.Fatal("expected an error")
	}

	deleted := media.deletedURLs(t)
	if len(deleted) != 1 {
		t.Fatalf("deleted = %v, want the new object removed", deleted)
	}
	if !strings.Contains(deleted[0], "file-1") {
		t.Errorf("deleted %q, want the object just uploaded", deleted[0])
	}
}

// A type the endpoint does not accept is refused before anything is uploaded.
func TestUploadFileRejectsUnsupportedTypeBeforeUploading(t *testing.T) {
	store := &fakeStore{}
	svc, media := newService(t, store)

	if _, err := svc.UploadFile(context.Background(), "t", testID, "application/zip", []byte("PK\x03\x04"), nil, 4); err == nil {
		t.Fatal("expected an error for a zip")
	}
	if len(media.callsFor()) != 0 {
		t.Errorf("an unsupported type reached media-service: %v", media.callsFor())
	}
}

// ── the delete path ───────────────────────────────────────────────────────

// Deleting clears the row and removes the object.
func TestDeleteFileClearsRowAndRemovesObject(t *testing.T) {
	store := &fakeStore{current: "certificates/abc.pdf"}
	svc, media := newService(t, store)

	got, err := svc.DeleteFile(context.Background(), "t", testID)
	if err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if store.setKey != "" {
		t.Errorf("row key = %q, want it cleared", store.setKey)
	}
	if got.FileKey != "" {
		t.Errorf("returned key = %q, want empty", got.FileKey)
	}

	deleted := media.deletedURLs(t)
	if len(deleted) != 1 || deleted[0] != "certificates/abc.pdf" {
		t.Errorf("deleted = %v, want certificates/abc.pdf", deleted)
	}
}

// A failed object delete is reported: the caller asked for the file to be
// gone, and saying "deleted" while it survives would be a lie.
func TestDeleteFileReportsAFailedObjectDelete(t *testing.T) {
	store := &fakeStore{current: "certificates/abc.pdf"}
	svc, media := newService(t, store)
	media.failDeletes(http.StatusInternalServerError)

	got, err := svc.DeleteFile(context.Background(), "t", testID)
	if err == nil {
		t.Fatal("expected an error when the object could not be removed")
	}
	// The row is still returned so the dashboard can refresh.
	if got == nil {
		t.Error("got nil achievement, want the cleared row alongside the error")
	}
}

// Nothing to delete means no call at all.
func TestDeleteFileWithNoFileMakesNoCall(t *testing.T) {
	store := &fakeStore{current: ""}
	svc, media := newService(t, store)

	if _, err := svc.DeleteFile(context.Background(), "t", testID); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if len(media.callsFor()) != 0 {
		t.Errorf("calls = %v, want none", media.callsFor())
	}
}

// ── media-service unavailable ─────────────────────────────────────────────

// A 5xx from media-service is an outage: the upload did not happen and the
// caller should retry, not change the file.
func TestUploadFileMediaServiceDownIsAnOutage(t *testing.T) {
	store := &fakeStore{}
	svc, media := newService(t, store)
	media.failUploads(http.StatusInternalServerError, `{"success":false,"message":"boom"}`)

	_, err := svc.UploadFile(context.Background(), "t", testID, "application/pdf", pdfBytes, nil, int64(len(pdfBytes)))
	if !errors.Is(err, ErrMediaUnavailable) {
		t.Errorf("err = %v, want ErrMediaUnavailable", err)
	}
	if store.setKey != "" {
		t.Errorf("row was written despite the outage: %q", store.setKey)
	}
}

// An unreachable host is an outage too, not a panic or a hang.
func TestUploadFileUnreachableMediaServiceIsAnOutage(t *testing.T) {
	svc := &AchievementService{media: NewMediaClient("http://127.0.0.1:1"), store: &fakeStore{}}

	_, err := svc.UploadFile(context.Background(), "t", testID, "application/pdf", pdfBytes, nil, int64(len(pdfBytes)))
	if !errors.Is(err, ErrMediaUnavailable) {
		t.Errorf("err = %v, want ErrMediaUnavailable", err)
	}
}

// A file media-service refuses is a rejection, not an outage: the two need
// different messages and different retry advice.
func TestUploadFileRejectionIsNotAnOutage(t *testing.T) {
	store := &fakeStore{}
	svc, media := newService(t, store)
	media.failUploads(http.StatusUnsupportedMediaType,
		`{"success":false,"message":"format must be JPEG, PNG, WebP, AVIF or SVG","error":{"code":"UNSUPPORTED_TYPE"}}`)

	_, err := svc.UploadFile(context.Background(), "t", testID, "application/pdf", pdfBytes, nil, int64(len(pdfBytes)))

	var rejected *ErrMediaRejected
	if !errors.As(err, &rejected) {
		t.Fatalf("err = %v (%T), want *ErrMediaRejected", err, err)
	}
	if rejected.Status != http.StatusUnsupportedMediaType || rejected.Code != "UNSUPPORTED_TYPE" {
		t.Errorf("rejected = %+v, want 415/UNSUPPORTED_TYPE", rejected)
	}
	if errors.Is(err, ErrMediaUnavailable) {
		t.Error("a rejection must not also read as an outage")
	}
}

// An unset MEDIA_SERVICE_URL disables uploads with a clear error rather than
// failing at boot.
func TestUnconfiguredMediaClient(t *testing.T) {
	client := NewMediaClient("")
	if client.Configured() {
		t.Error("empty base URL must not count as configured")
	}
	if _, _, err := client.Upload(context.Background(), "t", "certificates", "id", "file", pdfBytes); !errors.Is(err, ErrMediaUnavailable) {
		t.Errorf("err = %v, want ErrMediaUnavailable", err)
	}
	if err := client.Delete(context.Background(), "t", "certificates/x.pdf"); !errors.Is(err, ErrMediaUnavailable) {
		t.Errorf("delete err = %v, want ErrMediaUnavailable", err)
	}
}
