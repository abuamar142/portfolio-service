// Package services — the client for media-service.
//
// Certificate files live in media-service, not here: it owns the R2
// credentials, the per-project bucket, and the format allowlist. This service
// only decides which achievement a file belongs to and stores what it gets
// back.
//
// Keeping the credentials out of this process is the point. A token that leaks
// here cannot reach another project's bucket, because this service never holds
// one.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrMediaUnavailable means the media service could not be reached or
// answered with a server error. Handlers map it to 503: the upload did not
// happen, and the caller should retry rather than fix its request.
var ErrMediaUnavailable = errors.New("media service unavailable")

// ErrMediaRejected means media-service refused the file itself (too large,
// wrong type, empty). It carries the client-facing reason so the dashboard can
// show what was wrong with the file rather than a generic failure.
type ErrMediaRejected struct {
	// Status is the status media-service answered with.
	Status int
	// Code is its machine-readable error code, e.g. UNSUPPORTED_TYPE.
	Code string
	// Message is the human-readable reason it returned.
	Message string
}

func (e *ErrMediaRejected) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("media service rejected the file: %s", e.Code)
	}
	return fmt.Sprintf("media service rejected the file (status %d)", e.Status)
}

// MediaProject is the project name media-service knows this data under. It
// selects the bucket (portfolio-assets) on that side.
const MediaProject = "portfolio"

// MediaClient calls media-service.
type MediaClient struct {
	baseURL string
	http    *http.Client
}

// NewMediaClient builds a client. An empty baseURL means uploads are
// unavailable, which the handler reports as 503 rather than failing at boot —
// every read and every other write keeps working without it.
func NewMediaClient(baseURL string) *MediaClient {
	return &MediaClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		// Bounded: a hung media-service must fail the upload, not pin the
		// goroutine and the caller's connection. A certificate is up to
		// 10 MB, so this is generous for the transfer while still finite.
		http: &http.Client{Timeout: 60 * time.Second},
	}
}

// Configured reports whether a base URL was supplied.
func (c *MediaClient) Configured() bool {
	return c != nil && c.baseURL != ""
}

// Upload forwards the bytes to media-service and returns the stored public URL
// and its object key.
//
// The key is returned alongside the URL because the database stores the key
// and the dashboard builds the link itself (FILES_URL + "/" + key). Storing
// the URL instead would produce a doubled prefix on every certificate link.
//
// The bearer token is passed through unchanged: media-service validates it
// itself, so an upload is authorised by the same token that authorised the
// request here. Re-using it rather than minting a service token means a
// revoked user cannot upload either.
func (c *MediaClient) Upload(ctx context.Context, token, resource, id, field string, body []byte) (publicURL, key string, err error) {
	if !c.Configured() {
		return "", "", ErrMediaUnavailable
	}

	endpoint := fmt.Sprintf("%s/api/v1/media/%s/upload?resource=%s&id=%s&field=%s",
		c.baseURL, url.PathEscape(MediaProject),
		url.QueryEscape(resource), url.QueryEscape(id), url.QueryEscape(field))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("building upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	// The body is raw bytes; media-service sniffs the type and ignores this
	// header for the decision. Sent only so its error messages can name what
	// the client claimed.
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrMediaUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", "", mediaError(resp)
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			URL string `json:"url"`
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&envelope); err != nil {
		return "", "", fmt.Errorf("%w: decoding response: %v", ErrMediaUnavailable, err)
	}
	if !envelope.Success || envelope.Data.URL == "" || envelope.Data.Key == "" {
		return "", "", fmt.Errorf("%w: media service returned no url/key", ErrMediaUnavailable)
	}
	return envelope.Data.URL, envelope.Data.Key, nil
}

// Delete removes one object by its public URL.
//
// Best-effort by contract: the caller has already updated the row, so a
// failure here leaves an object nothing points at. That is worth logging, but
// it must never undo a successful row update.
func (c *MediaClient) Delete(ctx context.Context, token, publicURL string) error {
	if !c.Configured() {
		return ErrMediaUnavailable
	}
	if strings.TrimSpace(publicURL) == "" {
		return nil
	}

	payload, err := json.Marshal(map[string]string{"url": publicURL})
	if err != nil {
		return fmt.Errorf("encoding delete request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/media/%s/object", c.baseURL, url.PathEscape(MediaProject))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("building delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMediaUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mediaError(resp)
	}
	return nil
}

// mediaError turns a non-2xx answer into the right error type.
//
// A 4xx is the file's fault and is reported as such; a 5xx or an unreadable
// body is the service's, and becomes ErrMediaUnavailable so the caller is
// told to retry rather than to change the file.
func mediaError(resp *http.Response) error {
	var envelope struct {
		Message string `json:"message"`
		Error   struct {
			Code    string `json:"code"`
			Details string `json:"details"`
		} `json:"error"`
	}
	// The body is only read to make the message useful; a failure to parse it
	// must not change the classification.
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&envelope)

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return &ErrMediaRejected{
			Status:  resp.StatusCode,
			Code:    envelope.Error.Code,
			Message: envelope.Message,
		}
	}
	return fmt.Errorf("%w: status %d", ErrMediaUnavailable, resp.StatusCode)
}
