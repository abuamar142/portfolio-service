package game

import (
	crand "crypto/rand"
	"encoding/binary"
	mrand "math/rand/v2"
	"sync"
	"time"
)

// Manager owns the rooms.
//
// State is in memory on purpose. A room lives minutes, is worthless after the
// last player leaves, and every read is a write away from the previous one — a
// table plus 20 Hz updates would buy nothing but contention. The tradeoff is
// explicit: a service restart ends live games, so the client treats a missing
// room as "back to the lobby" rather than as an error.
type Manager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
	bank  *Bank
	rng   *mrand.Rand
}

// NewManager builds a manager over a word bank.
func NewManager(bank *Bank) *Manager {
	return &Manager{
		rooms: make(map[string]*Room),
		bank:  bank,
		rng:   mrand.New(mrand.NewPCG(seededUint64(), seededUint64())),
	}
}

// seededUint64 makes the word order unpredictable. Predictability here is not a
// security hole, but a player who knows the next word stops playing the game.
func seededUint64() uint64 {
	var buf [8]byte
	if _, err := crand.Read(buf[:]); err != nil {
		return uint64(time.Now().UnixNano())
	}
	return binary.LittleEndian.Uint64(buf[:])
}

// Create opens a room, seating the host as its first player.
func (m *Manager) Create(name string, settings Settings) (*Room, Player, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room := newRoom(m.freeCodeLocked(), m.bank, m.rng, settings.normalized())
	player, err := room.addPlayerLocked(name)
	if err != nil {
		return nil, Player{}, err
	}
	room.hostID = player.ID
	m.rooms[room.code] = room
	return room, player, nil
}

// Get looks a room up by code. The code is uppercased so a pasted "abcd" works.
func (m *Manager) Get(code string) (*Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.rooms[normalizeCode(code)]
	if !ok {
		return nil, ErrNotFound
	}
	return room, nil
}

// Join seats another player in an existing room.
func (m *Manager) Join(code, name string) (*Room, Player, error) {
	room, err := m.Get(code)
	if err != nil {
		return nil, Player{}, err
	}
	player, err := room.AddPlayer(name)
	if err != nil {
		return nil, Player{}, err
	}
	return room, player, nil
}

// Leave removes a player. In the lobby that deletes the seat; once the game has
// started it only frees the name, because erasing a player mid-race would
// rewrite the scoreboard under everyone else.
func (m *Manager) Leave(code, playerID string) error {
	room, err := m.Get(code)
	if err != nil {
		return err
	}
	room.removePlayer(playerID)
	return nil
}

// Sweep drops rooms nobody has touched. Called periodically from main.
func (m *Manager) Sweep(idle time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-idle)
	removed := 0
	for code, room := range m.rooms {
		if room.LastActive().Before(cutoff) {
			room.close()
			delete(m.rooms, code)
			removed++
		}
	}
	return removed
}

// Len reports the live room count (used by tests and by a health readout).
func (m *Manager) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms)
}

func (m *Manager) freeCodeLocked() string {
	for {
		code := newCode()
		if _, taken := m.rooms[code]; !taken {
			return code
		}
	}
}

// normalizeCode uppercases and strips separators, so "ab-cd" and "ABCD" reach
// the same room.
func normalizeCode(code string) string {
	out := make([]byte, 0, len(code))
	for _, r := range code {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, byte(r-32))
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, byte(r))
		}
	}
	return string(out)
}

func (r *Room) removePlayer(playerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	index := r.playerIndexLocked(playerID)
	if index < 0 {
		return
	}
	if r.phase != PhaseLobby {
		return
	}
	r.players = append(r.players[:index], r.players[index+1:]...)
	if r.hostID == playerID {
		r.hostID = ""
		if len(r.players) > 0 {
			r.hostID = r.players[0].ID
		}
	}
	r.lastActive = time.Now()
	r.publishLocked()
}

// close releases every subscriber so idle rooms do not keep connections open.
func (r *Room) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, ch := range r.subs {
		delete(r.subs, id)
		close(ch)
	}
}
