package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/abuamar142/portfolio-service/internal/game"
	"github.com/abuamar142/portfolio-service/internal/response"
)

// GameHandler serves the anagram party game.
//
// Identity is the player id handed out at create/join: a 128-bit random string
// the client sends back on every action. There is no account and no cookie, so
// possession of the id *is* the credential. That is the right trade for a party
// game people join from three devices in one room — anyone with the id is
// already in the room — and it is not a credential for anything else.
type GameHandler struct {
	manager *game.Manager
}

func NewGameHandler(manager *game.Manager) *GameHandler {
	return &GameHandler{manager: manager}
}

type createRoomRequest struct {
	Name     string        `json:"name"`
	Settings game.Settings `json:"settings"`
}

type createRoomResponse struct {
	Code     string        `json:"code"`
	PlayerID string        `json:"playerId"`
	State    game.Snapshot `json:"state"`
}

type joinRoomResponse struct {
	PlayerID string        `json:"playerId"`
	State    game.Snapshot `json:"state"`
}

type playerRequest struct {
	PlayerID string `json:"playerId"`
}

type guessRequest struct {
	PlayerID string `json:"playerId"`
	Guess    string `json:"guess"`
}

type scoreRequest struct {
	PlayerID string `json:"playerId"`
	TargetID string `json:"targetId"`
	Delta    int    `json:"delta"`
}

type settingsRequest struct {
	PlayerID string        `json:"playerId"`
	Settings game.Settings `json:"settings"`
}

// Create opens a room and seats the caller as host.
func (h *GameHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest
	if !decode(w, r, &req) {
		return
	}

	room, player, err := h.manager.Create(req.Name, req.Settings)
	if err != nil {
		fail(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, "room created", createRoomResponse{
		Code:     room.Code(),
		PlayerID: player.ID,
		State:    room.Snapshot(),
	})
}

// Join seats the caller in an existing room.
func (h *GameHandler) Join(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest
	if !decode(w, r, &req) {
		return
	}

	room, player, err := h.manager.Join(chi.URLParam(r, "code"), req.Name)
	if err != nil {
		fail(w, err)
		return
	}

	response.JSON(w, http.StatusOK, "joined", joinRoomResponse{
		PlayerID: player.ID,
		State:    room.Snapshot(),
	})
}

// Get returns the current room state.
//
// The SSE stream also sends this as its first frame, but a client restoring a
// saved session needs a plain request: an EventSource aimed at a room that no
// longer exists retries forever instead of reporting the miss.
func (h *GameHandler) Get(w http.ResponseWriter, r *http.Request) {
	room, ok := h.room(w, r)
	if !ok {
		return
	}
	response.JSON(w, http.StatusOK, "ok", room.Snapshot())
}

// Guess submits an answer for the current round.
func (h *GameHandler) Guess(w http.ResponseWriter, r *http.Request) {
	room, ok := h.room(w, r)
	if !ok {
		return
	}
	var req guessRequest
	if !decode(w, r, &req) {
		return
	}

	result, err := room.Submit(req.PlayerID, req.Guess)
	if err != nil {
		fail(w, err)
		return
	}
	response.JSON(w, http.StatusOK, "guess recorded", result)
}

// Start begins the game.
func (h *GameHandler) Start(w http.ResponseWriter, r *http.Request) {
	h.hostAction(w, r, func(room *game.Room, playerID string) error {
		return room.Start(playerID)
	})
}

// Next skips the reveal pause.
func (h *GameHandler) Next(w http.ResponseWriter, r *http.Request) {
	h.hostAction(w, r, func(room *game.Room, playerID string) error {
		return room.Next(playerID)
	})
}

// Restart starts a fresh game in the same room.
func (h *GameHandler) Restart(w http.ResponseWriter, r *http.Request) {
	h.hostAction(w, r, func(room *game.Room, playerID string) error {
		return room.Restart(playerID)
	})
}

// UpdateSettings changes the rules before the game starts.
func (h *GameHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	room, ok := h.room(w, r)
	if !ok {
		return
	}
	var req settingsRequest
	if !decode(w, r, &req) {
		return
	}
	if err := room.SetSettings(req.PlayerID, req.Settings); err != nil {
		fail(w, err)
		return
	}
	response.JSON(w, http.StatusOK, "settings updated", room.Snapshot())
}

// AdjustScore applies the host's manual +/- correction.
func (h *GameHandler) AdjustScore(w http.ResponseWriter, r *http.Request) {
	room, ok := h.room(w, r)
	if !ok {
		return
	}
	var req scoreRequest
	if !decode(w, r, &req) {
		return
	}
	if err := room.AdjustScore(req.PlayerID, req.TargetID, req.Delta); err != nil {
		fail(w, err)
		return
	}
	response.JSON(w, http.StatusOK, "score adjusted", room.Snapshot())
}

// Leave frees a seat in the lobby.
func (h *GameHandler) Leave(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if err := h.manager.Leave(code, chi.URLParam(r, "playerID")); err != nil {
		fail(w, err)
		return
	}
	response.JSON(w, http.StatusOK, "left", nil)
}

// Stream is the server-sent event feed.
//
// SSE rather than polling because every player must see the same scramble at
// the same instant: with polling, a 900 ms interval spreads the phase change
// across ±450 ms, which is 15–20% of the time it takes to solve a five-letter
// anagram. Answers still arrive over POST, so the server decides the winner
// from arrival order and never from a timestamp a client supplied.
func (h *GameHandler) Stream(w http.ResponseWriter, r *http.Request) {
	room, ok := h.room(w, r)
	if !ok {
		return
	}

	flusher, canFlush := w.(http.Flusher)
	if !canFlush {
		response.Error(w, http.StatusInternalServerError, "streaming_unsupported", "streaming is not supported", "")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	// nginx buffers proxied responses by default, which would hold every frame
	// until the buffer fills — the whole point of SSE is not doing that.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	events, cancel := room.Subscribe()
	defer cancel()

	// First frame is the current state, so a late joiner or a reconnect after a
	// dropped connection is correct without a separate fetch.
	writeEvent(w, flusher, mustJSON(room.Snapshot()))

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case payload, open := <-events:
			if !open {
				// Room swept or closed: tell the client so it can fall back to
				// the lobby instead of reconnecting forever.
				fmt.Fprint(w, "event: closed\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			writeEvent(w, flusher, payload)
		case <-keepAlive.C:
			// Comment frames keep proxies from timing out an idle stream.
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// room resolves the {code} parameter or writes the error.
func (h *GameHandler) room(w http.ResponseWriter, r *http.Request) (*game.Room, bool) {
	room, err := h.manager.Get(chi.URLParam(r, "code"))
	if err != nil {
		fail(w, err)
		return nil, false
	}
	return room, true
}

// hostAction runs one host-only mutation and answers with the new state.
func (h *GameHandler) hostAction(w http.ResponseWriter, r *http.Request, fn func(*game.Room, string) error) {
	room, ok := h.room(w, r)
	if !ok {
		return
	}
	var req playerRequest
	if !decode(w, r, &req) {
		return
	}
	if err := fn(room, req.PlayerID); err != nil {
		fail(w, err)
		return
	}
	response.JSON(w, http.StatusOK, "ok", room.Snapshot())
}

// writeEvent emits one named SSE frame.
//
// The event name is not optional: an unlabelled frame is dispatched as a
// generic `message`, and a client listening on `state` would silently ignore
// every update after the first one. Naming every frame is what keeps the
// stream and its listener in agreement.
func writeEvent(w http.ResponseWriter, flusher http.Flusher, payload []byte) {
	fmt.Fprintf(w, "event: state\ndata: %s\n\n", payload)
	flusher.Flush()
}

func mustJSON(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return payload
}

// decode reads a JSON body and writes a 400 when it is malformed.
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	// 1 MiB is far more than any request here needs; the limit exists so a
	// hostile body cannot make the service allocate without bound.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON", err.Error())
		return false
	}
	return true
}

// fail maps engine errors onto status codes. The client keys its UI off these
// codes, so they are part of the contract rather than an implementation detail.
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, game.ErrNotFound):
		response.Error(w, http.StatusNotFound, "room_not_found", "that room code does not exist", "")
	case errors.Is(err, game.ErrRoomFull):
		response.Error(w, http.StatusConflict, "room_full", "this room already has the maximum number of players", "")
	case errors.Is(err, game.ErrNameTaken):
		response.Error(w, http.StatusConflict, "name_taken", "another player in this room already has that name", "")
	case errors.Is(err, game.ErrInvalidName):
		response.Error(w, http.StatusBadRequest, "invalid_name", "a player name is required", "")
	case errors.Is(err, game.ErrNotHost):
		response.Error(w, http.StatusForbidden, "not_host", "only the host can do that", "")
	case errors.Is(err, game.ErrWrongPhase):
		response.Error(w, http.StatusConflict, "wrong_phase", "that action is not available right now", "")
	case errors.Is(err, game.ErrPlayerNotFound):
		response.Error(w, http.StatusNotFound, "player_not_found", "that player is not in this room", "")
	case errors.Is(err, game.ErrLocked):
		response.Error(w, http.StatusTooManyRequests, "locked_out", "you have no attempts left this round", "")
	case errors.Is(err, game.ErrEmptyGuess):
		response.Error(w, http.StatusBadRequest, "empty_guess", "the guess is empty", "")
	case errors.Is(err, game.ErrNoWords):
		response.Error(w, http.StatusServiceUnavailable, "no_words", "the word bank has no words for that language", "")
	default:
		response.Error(w, http.StatusInternalServerError, "internal_error", "something went wrong", "")
	}
}
