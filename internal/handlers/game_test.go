package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/abuamar142/portfolio-service/internal/game"
)

// These tests go through the router, not the engine: the contract the frontend
// depends on is "POST this, get that state back", and the engine's own tests
// already cover the rules.

func newGameRouter(t *testing.T) (chi.Router, *game.Manager) {
	t.Helper()
	return newGameRouterWithBank(t, game.NewBank(game.DefaultEntries()))
}

// newGameRouterWithBank builds the router over a chosen bank. Tests that must
// know the answer use a one-word bank instead of reaching into the engine: the
// server never exposes the answer to a client, so neither should a test seam.
func newGameRouterWithBank(t *testing.T, bank *game.Bank) (chi.Router, *game.Manager) {
	t.Helper()
	manager := game.NewManager(bank)
	handler := NewGameHandler(manager)

	r := chi.NewRouter()
	r.Route("/api/v1/game", func(r chi.Router) {
		r.Post("/rooms", handler.Create)
		r.Get("/rooms/{code}/events", handler.Stream)
		r.Post("/rooms/{code}/join", handler.Join)
		r.Post("/rooms/{code}/start", handler.Start)
		r.Post("/rooms/{code}/next", handler.Next)
		r.Post("/rooms/{code}/restart", handler.Restart)
		r.Post("/rooms/{code}/settings", handler.UpdateSettings)
		r.Post("/rooms/{code}/guess", handler.Guess)
		r.Post("/rooms/{code}/score", handler.AdjustScore)
		r.Delete("/rooms/{code}/players/{playerID}", handler.Leave)
	})
	return r, manager
}

type envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code string `json:"code"`
	} `json:"error"`
}

func call(t *testing.T, r chi.Router, method, path string, body any) (int, envelope) {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("%s %s: body is not JSON: %s", method, path, rec.Body.String())
	}
	return rec.Code, env
}

type roomState struct {
	Code     string        `json:"code"`
	Phase    string        `json:"phase"`
	HostID   string        `json:"hostId"`
	Players  []game.Player `json:"players"`
	Settings game.Settings `json:"settings"`
	Target   int           `json:"target"`
	Solved   int           `json:"solved"`
	Puzzle   *struct {
		Scrambled string `json:"scrambled"`
		Category  string `json:"category"`
		Length    int    `json:"length"`
	} `json:"puzzle"`
	Reveal string `json:"reveal"`
}

func TestCreateRoomReturnsCodeAndState(t *testing.T) {
	r, _ := newGameRouter(t)

	status, env := call(t, r, http.MethodPost, "/api/v1/game/rooms", map[string]any{
		"name":     "Amar",
		"settings": map[string]any{"language": "id"},
	})
	if status != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", status, env.Message)
	}

	var created struct {
		Code     string    `json:"code"`
		PlayerID string    `json:"playerId"`
		State    roomState `json:"state"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Code) != 4 {
		t.Fatalf("code = %q, want 4 characters", created.Code)
	}
	if created.State.Phase != game.PhaseLobby {
		t.Fatalf("phase = %s, want lobby", created.State.Phase)
	}
	if created.State.HostID != created.PlayerID {
		t.Fatal("creator is not the host")
	}
	if created.State.Players[0].Name != "Amar" {
		t.Fatalf("name = %q", created.State.Players[0].Name)
	}
}

func TestJoinAddsPlayerAndRejectsDuplicates(t *testing.T) {
	r, _ := newGameRouter(t)
	code, _ := createRoom(t, r, "Amar")

	status, env := call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/join", map[string]any{"name": "Budi"})
	if status != http.StatusOK {
		t.Fatalf("join status = %d (%s)", status, env.Message)
	}

	status, env = call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/join", map[string]any{"name": "BUDI"})
	if status != http.StatusConflict || env.Error == nil || env.Error.Code != "name_taken" {
		t.Fatalf("duplicate join = %d %+v", status, env.Error)
	}
}

func TestUnknownRoomIs404(t *testing.T) {
	r, _ := newGameRouter(t)
	status, env := call(t, r, http.MethodPost, "/api/v1/game/rooms/ZZZZ/join", map[string]any{"name": "Budi"})
	if status != http.StatusNotFound || env.Error == nil || env.Error.Code != "room_not_found" {
		t.Fatalf("status = %d, error = %+v", status, env.Error)
	}
}

func TestRoomCodeLookupIsCaseInsensitive(t *testing.T) {
	r, _ := newGameRouter(t)
	code, _ := createRoom(t, r, "Amar")
	status, _ := call(t, r, http.MethodPost, "/api/v1/game/rooms/"+strings.ToLower(code)+"/join", map[string]any{"name": "Budi"})
	if status != http.StatusOK {
		t.Fatalf("lowercase code join = %d", status)
	}
}

func TestStartRequiresHost(t *testing.T) {
	r, _ := newGameRouter(t)
	code, hostID := createRoom(t, r, "Amar")
	guestID := joinRoom(t, r, code, "Budi")

	status, env := call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/start", map[string]any{"playerId": guestID})
	if status != http.StatusForbidden || env.Error == nil || env.Error.Code != "not_host" {
		t.Fatalf("guest start = %d %+v", status, env.Error)
	}

	status, env = call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/start", map[string]any{"playerId": hostID})
	if status != http.StatusOK {
		t.Fatalf("host start = %d (%s)", status, env.Message)
	}
}

func TestGuessReturnsPointsAndRevealOnSuccess(t *testing.T) {
	// One word in the bank means the test knows the answer without asking.
	bank := game.NewBank(map[string][]game.Entry{"id": {{Word: "buku", Category: "benda"}}})
	r, manager := newGameRouterWithBank(t, bank)
	code, hostID := createRoom(t, r, "Amar")
	if status, _ := call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/start", map[string]any{"playerId": hostID}); status != http.StatusOK {
		t.Fatal("start failed")
	}

	const answer = "BUKU"
	status, env := call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/guess", map[string]any{
		"playerId": hostID,
		"guess":    answer,
	})
	if status != http.StatusOK {
		t.Fatalf("guess status = %d (%s)", status, env.Message)
	}

	var result struct {
		Correct bool `json:"correct"`
		Points  int  `json:"points"`
	}
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatal(err)
	}
	if !result.Correct || result.Points < 1 {
		t.Fatalf("result = %+v", result)
	}

	state := currentState(t, r, manager, code)
	if state.Phase != game.PhaseReveal || state.Reveal != answer {
		t.Fatalf("state = %s reveal = %s", state.Phase, state.Reveal)
	}
}

func TestWrongGuessReportsLockout(t *testing.T) {
	bank := game.NewBank(map[string][]game.Entry{"id": {{Word: "buku", Category: "benda"}}})
	r, _ := newGameRouterWithBank(t, bank)
	code, hostID := createRoomWithSettings(t, r, "Amar", map[string]any{"maxWrong": 1})
	if status, _ := call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/start", map[string]any{"playerId": hostID}); status != http.StatusOK {
		t.Fatal("start failed")
	}

	status, env := call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/guess", map[string]any{
		"playerId": hostID,
		"guess":    "ZZZZZ",
	})
	if status != http.StatusOK {
		t.Fatalf("wrong guess status = %d (%s)", status, env.Message)
	}
	var result struct {
		Correct bool `json:"correct"`
		Locked  bool `json:"locked"`
	}
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Correct || !result.Locked {
		t.Fatalf("result = %+v, want correct=false locked=true", result)
	}

	// A second attempt in the same round is refused, not counted.
	status, env = call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/guess", map[string]any{
		"playerId": hostID,
		"guess":    "BUKU",
	})
	if status != http.StatusTooManyRequests || env.Error == nil || env.Error.Code != "locked_out" {
		t.Fatalf("locked submit = %d %+v", status, env.Error)
	}
}

func TestAdjustScoreIsHostOnly(t *testing.T) {
	r, manager := newGameRouter(t)
	code, hostID := createRoom(t, r, "Amar")
	guestID := joinRoom(t, r, code, "Budi")

	status, env := call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/score", map[string]any{
		"playerId": guestID, "targetId": guestID, "delta": 2,
	})
	if status != http.StatusForbidden || env.Error == nil || env.Error.Code != "not_host" {
		t.Fatalf("guest adjust = %d %+v", status, env.Error)
	}

	status, _ = call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/score", map[string]any{
		"playerId": hostID, "targetId": guestID, "delta": 2,
	})
	if status != http.StatusOK {
		t.Fatalf("host adjust = %d", status)
	}
	state := currentState(t, r, manager, code)
	if state.Players[1].Score != 2 {
		t.Fatalf("guest score = %d, want 2", state.Players[1].Score)
	}
}

func TestSettingsArePersisted(t *testing.T) {
	r, manager := newGameRouter(t)
	code, hostID := createRoom(t, r, "Amar")

	status, _ := call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/settings", map[string]any{
		"playerId": hostID,
		"settings": map[string]any{"language": "en", "maxWrong": 5, "target": 4},
	})
	if status != http.StatusOK {
		t.Fatalf("settings status = %d", status)
	}

	state := currentState(t, r, manager, code)
	if state.Settings.Language != "en" || state.Settings.MaxWrong != 5 {
		t.Fatalf("settings = %+v", state.Settings)
	}
	// An explicit target survives its own round trip.
	if status, _ := call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/start", map[string]any{"playerId": hostID}); status != http.StatusOK {
		t.Fatal("start failed")
	}
	if target := currentState(t, r, manager, code).Target; target != 4 {
		t.Fatalf("target = %d, want 4", target)
	}
}

func TestMalformedBodyIs400(t *testing.T) {
	r, _ := newGameRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/game/rooms", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// Stream must send the current state immediately: a client that reconnects has
// no other way to know the phase, and polling alongside the stream would defeat
// the point of using one.
func TestStreamSendsInitialState(t *testing.T) {
	r, _ := newGameRouter(t)
	code, _ := createRoom(t, r, "Amar")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/game/rooms/"+code+"/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		r.ServeHTTP(rec, req)
		close(done)
	}()

	// Wait for the first frame rather than for the handler to return: the
	// stream stays open by design.
	deadline := time.After(1500 * time.Millisecond)
	for {
		body := rec.Body.String()
		if strings.Contains(body, "data: ") {
			if !strings.Contains(body, `"phase":"lobby"`) {
				t.Fatalf("first frame missing lobby state: %s", body)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("no event within timeout, body = %q", body)
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()
	<-done
}

// --- helpers -----------------------------------------------------------------

func mustRoom(t *testing.T, manager *game.Manager, code string) *game.Room {
	t.Helper()
	room, err := manager.Get(code)
	if err != nil {
		t.Fatal(err)
	}
	return room
}

func currentState(t *testing.T, _ chi.Router, manager *game.Manager, code string) roomState {
	t.Helper()
	snapshot := mustRoom(t, manager, code).Snapshot()
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var state roomState
	if err := json.Unmarshal(payload, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func createRoom(t *testing.T, r chi.Router, name string) (string, string) {
	t.Helper()
	status, env := call(t, r, http.MethodPost, "/api/v1/game/rooms", map[string]any{"name": name})
	if status != http.StatusCreated {
		t.Fatalf("create = %d (%s)", status, env.Message)
	}
	return decodeCreated(t, env)
}

func createRoomWithSettings(t *testing.T, r chi.Router, name string, settings map[string]any) (string, string) {
	t.Helper()
	status, env := call(t, r, http.MethodPost, "/api/v1/game/rooms", map[string]any{"name": name, "settings": settings})
	if status != http.StatusCreated {
		t.Fatalf("create = %d (%s)", status, env.Message)
	}
	return decodeCreated(t, env)
}

func decodeCreated(t *testing.T, env envelope) (string, string) {
	t.Helper()
	var created struct {
		Code     string `json:"code"`
		PlayerID string `json:"playerId"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	return created.Code, created.PlayerID
}

func joinRoom(t *testing.T, r chi.Router, code, name string) string {
	t.Helper()
	status, env := call(t, r, http.MethodPost, "/api/v1/game/rooms/"+code+"/join", map[string]any{"name": name})
	if status != http.StatusOK {
		t.Fatalf("join = %d (%s)", status, env.Message)
	}
	var joined struct {
		PlayerID string `json:"playerId"`
	}
	if err := json.Unmarshal(env.Data, &joined); err != nil {
		t.Fatal(err)
	}
	return joined.PlayerID
}
