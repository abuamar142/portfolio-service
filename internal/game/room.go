package game

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	mrand "math/rand/v2"
	"strings"
	"sync"
	"time"
)

// Phases of one room. `reveal` exists so the answer and the scorer are visible
// before the next scramble replaces them; without it the round would end and
// vanish in the same broadcast.
const (
	PhaseLobby    = "lobby"
	PhaseRound    = "round"
	PhaseReveal   = "reveal"
	PhaseFinished = "finished"
)

// MaxPlayers bounds a room. Above this the scoreboard stops being readable on
// one screen and every broadcast gets heavier.
const MaxPlayers = 12

var (
	ErrNotFound       = errors.New("room not found")
	ErrRoomFull       = errors.New("room is full")
	ErrNameTaken      = errors.New("name already taken")
	ErrInvalidName    = errors.New("name is required")
	ErrNotHost        = errors.New("only the host can do that")
	ErrWrongPhase     = errors.New("not available in this phase")
	ErrPlayerNotFound = errors.New("player not in this room")
	ErrLocked         = errors.New("no attempts left this round")
	ErrEmptyGuess     = errors.New("guess is empty")
	ErrNoWords        = errors.New("word bank is empty for this language")
)

// Settings are chosen by the host when the room is created.
type Settings struct {
	Language     string `json:"language"`
	Target       int    `json:"target"`       // 0 = auto: max(5, players*2)
	MaxWrong     int    `json:"maxWrong"`     // wrong guesses before lockout, 1..10
	RoundSeconds int    `json:"roundSeconds"` // per-round time limit
}

func (s Settings) normalized() Settings {
	if s.Language != "en" {
		s.Language = "id"
	}
	if s.MaxWrong < 1 {
		s.MaxWrong = 3
	}
	if s.MaxWrong > 10 {
		s.MaxWrong = 10
	}
	if s.RoundSeconds < 10 {
		s.RoundSeconds = 30
	}
	if s.RoundSeconds > 180 {
		s.RoundSeconds = 180
	}
	if s.Target < 0 {
		s.Target = 0
	}
	if s.Target > 50 {
		s.Target = 50
	}
	return s
}

// targetFor resolves the win condition against the roster actually present.
//
// The host's explicit target wins; otherwise a room scales to twice its
// players with a floor of five, so five words are not fought over by eight
// people — in a race that means most of the room finishes on zero.
func targetFor(explicit int, players int) int {
	if explicit > 0 {
		return explicit
	}
	if players*2 > 5 {
		return players * 2
	}
	return 5
}

// Player is the public view of a participant. Wrong is the number of failed
// guesses in the current round; it is broadcast because the scoreboard already
// shows who is stuck, and hiding it only makes the lockout feel arbitrary.
type Player struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Score int    `json:"score"`
	Wrong int    `json:"wrong"`
}

// PuzzleView is everything a client may know about the current word before it
// is solved. The answer is deliberately absent.
type PuzzleView struct {
	Scrambled   string `json:"scrambled"`
	Category    string `json:"category"`
	Length      int    `json:"length"`
	SecondsLeft int    `json:"secondsLeft"`
}

// Snapshot is the whole room as seen by any player. One payload is broadcast to
// every subscriber; a player's own identity is known client-side, so nothing
// here needs to be per-viewer.
type Snapshot struct {
	Code        string      `json:"code"`
	Phase       string      `json:"phase"`
	HostID      string      `json:"hostId"`
	Players     []Player    `json:"players"`
	Settings    Settings    `json:"settings"`
	Target      int         `json:"target"`
	Solved      int         `json:"solved"`
	Round       int         `json:"round"`
	Puzzle      *PuzzleView `json:"puzzle,omitempty"`
	Reveal      string      `json:"reveal,omitempty"`
	RoundWinner string      `json:"roundWinner,omitempty"`
	Winner      string      `json:"winner,omitempty"`
}

// SubmitResult reports the outcome of one guess. A wrong guess is not an error:
// the client needs to know whether it was merely wrong or also the last try.
type SubmitResult struct {
	Correct bool `json:"correct"`
	Locked  bool `json:"locked"`
	Points  int  `json:"points"`
}

// Room is one game. All mutable state is behind mu; timers re-enter through
// mutate so every phase change broadcasts exactly once.
type Room struct {
	mu sync.Mutex

	code     string
	hostID   string
	phase    string
	players  []Player
	settings Settings
	target   int
	round    int
	solved   int
	used     map[string]bool

	puzzle    *Word
	scrambled string
	deadline  time.Time
	reveal    string
	roundWin  string
	winner    string

	// seq invalidates timers from a previous round. A timer that fires after
	// the host already advanced must not touch the current one.
	seq int

	lastActive  time.Time
	bank        *Bank
	rng         *mrand.Rand
	revealPause time.Duration

	subs    map[int]chan []byte
	nextSub int
}

func newRoom(code string, bank *Bank, rng *mrand.Rand, settings Settings) *Room {
	return &Room{
		code:        code,
		phase:       PhaseLobby,
		settings:    settings,
		used:        make(map[string]bool),
		bank:        bank,
		rng:         rng,
		revealPause: 2500 * time.Millisecond,
		lastActive:  time.Now(),
		subs:        make(map[int]chan []byte),
	}
}

// Code is the join code.
func (r *Room) Code() string { return r.code }

// Phase is the current phase (read under lock).
func (r *Room) Phase() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.phase
}

// LastActive reports when the room last did anything, for idle sweeping.
func (r *Room) LastActive() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastActive
}

// AddPlayer seats a new participant in the lobby.
func (r *Room) AddPlayer(name string) (Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.addPlayerLocked(name)
}

func (r *Room) addPlayerLocked(name string) (Player, error) {
	name = cleanName(name)
	if name == "" {
		return Player{}, ErrInvalidName
	}
	if r.phase != PhaseLobby {
		return Player{}, ErrWrongPhase
	}
	if len(r.players) >= MaxPlayers {
		return Player{}, ErrRoomFull
	}
	for _, existing := range r.players {
		if strings.EqualFold(existing.Name, name) {
			return Player{}, ErrNameTaken
		}
	}

	player := Player{ID: newID(), Name: name}
	r.players = append(r.players, player)
	r.hostID = chooseHost(r.hostID, player.ID)
	r.lastActive = time.Now()
	r.publishLocked()
	return player, nil
}

// chooseHost keeps the current host while they are still present, and otherwise
// promotes the first player — a lobby whose host left must not become unplayable.
func chooseHost(current, candidate string) string {
	if current != "" {
		return current
	}
	return candidate
}

// SetSettings changes the rules. Only the host, only before the game starts.
func (r *Room) SetSettings(playerID string, s Settings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if playerID != r.hostID {
		return ErrNotHost
	}
	if r.phase != PhaseLobby {
		return ErrWrongPhase
	}
	r.settings = s.normalized()
	r.lastActive = time.Now()
	r.publishLocked()
	return nil
}

// Start begins the first round.
func (r *Room) Start(playerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if playerID != r.hostID {
		return ErrNotHost
	}
	if r.phase != PhaseLobby {
		return ErrWrongPhase
	}
	if len(r.players) == 0 {
		return ErrPlayerNotFound
	}
	if r.bank.Len(r.settings.Language) == 0 {
		return ErrNoWords
	}
	r.resetForNewGameLocked()
	r.startRoundLocked()
	return nil
}

// Submit records a guess from one player.
func (r *Room) Submit(playerID, guess string) (SubmitResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.phase != PhaseRound {
		return SubmitResult{}, ErrWrongPhase
	}
	index := r.playerIndexLocked(playerID)
	if index < 0 {
		return SubmitResult{}, ErrPlayerNotFound
	}
	player := &r.players[index]
	if player.Wrong >= r.settings.MaxWrong {
		return SubmitResult{Locked: true}, ErrLocked
	}

	normalized := normalizeGuess(guess)
	if normalized == "" {
		return SubmitResult{}, ErrEmptyGuess
	}

	if !r.acceptsLocked(normalized) {
		player.Wrong++
		r.lastActive = time.Now()
		locked := player.Wrong >= r.settings.MaxWrong
		r.publishLocked()
		return SubmitResult{Correct: false, Locked: locked}, nil
	}

	points := pointsFor(len(r.puzzle.Answer))
	player.Score += points
	r.solved++
	r.reveal = r.puzzle.Answer
	r.roundWin = player.ID
	r.phase = PhaseReveal
	r.seq++
	r.scheduleAdvanceLocked()
	r.lastActive = time.Now()
	r.publishLocked()

	return SubmitResult{Correct: true, Points: points}, nil
}

func (r *Room) acceptsLocked(guess string) bool {
	for _, accepted := range r.puzzle.Accept {
		if guess == accepted {
			return true
		}
	}
	return false
}

// AdjustScore is the host's manual correction — the "+/-" on the scoreboard.
func (r *Room) AdjustScore(playerID, targetID string, delta int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if playerID != r.hostID {
		return ErrNotHost
	}
	index := r.playerIndexLocked(targetID)
	if index < 0 {
		return ErrPlayerNotFound
	}
	if delta == 0 || delta > 10 || delta < -10 {
		return ErrEmptyGuess
	}
	player := &r.players[index]
	player.Score += delta
	if player.Score < 0 {
		player.Score = 0
	}
	r.lastActive = time.Now()
	r.publishLocked()
	return nil
}

// Next lets the host cut the reveal pause short.
func (r *Room) Next(playerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if playerID != r.hostID {
		return ErrNotHost
	}
	if r.phase != PhaseReveal {
		return ErrWrongPhase
	}
	r.seq++
	r.advanceLocked()
	return nil
}

// Restart plays another game in the same room, keeping the roster.
//
// It resets everything a finished game accumulated — scores, solved count,
// round number and the used-word set. Keeping the roster but not the history is
// the whole point: the group stays together, the race starts over.
func (r *Room) Restart(playerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if playerID != r.hostID {
		return ErrNotHost
	}
	if r.phase != PhaseFinished {
		return ErrWrongPhase
	}
	r.resetForNewGameLocked()
	r.startRoundLocked()
	return nil
}

// resetForNewGameLocked clears per-game state. Shared by Start and Restart so
// the two entry points cannot drift apart.
func (r *Room) resetForNewGameLocked() {
	r.target = targetFor(r.settings.Target, len(r.players))
	r.round = 0
	r.solved = 0
	r.winner = ""
	r.reveal = ""
	r.roundWin = ""
	r.puzzle = nil
	r.used = make(map[string]bool)
	for i := range r.players {
		r.players[i].Score = 0
		r.players[i].Wrong = 0
	}
}

// Snapshot returns the room state for a new subscriber.
func (r *Room) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

// Subscribe returns a channel of marshalled snapshots. cancel must always be
// called — the SSE handler defers it.
func (r *Room) Subscribe() (<-chan []byte, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := r.nextSub
	r.nextSub++
	// Buffered so a broadcast never blocks on a slow reader; the client also
	// gets a fresh snapshot on reconnect, so dropping is recoverable.
	ch := make(chan []byte, 16)
	r.subs[id] = ch
	r.lastActive = time.Now()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			if existing, ok := r.subs[id]; ok {
				delete(r.subs, id)
				close(existing)
			}
		})
	}
	return ch, cancel
}

// --- internals ---------------------------------------------------------------

func (r *Room) playerIndexLocked(id string) int {
	for i := range r.players {
		if r.players[i].ID == id {
			return i
		}
	}
	return -1
}

func (r *Room) startRoundLocked() {
	word, ok := r.bank.Pick(r.settings.Language, r.used, r.rng)
	if !ok {
		// Every anagram group has been played: end rather than repeat.
		r.finishLocked()
		return
	}
	r.used[word.Key] = true
	r.round++
	r.puzzle = &word
	r.scrambled = Scramble(word.Answer, r.rng)
	r.reveal = ""
	r.roundWin = ""
	r.phase = PhaseRound
	r.deadline = time.Now().Add(time.Duration(r.settings.RoundSeconds) * time.Second)
	for i := range r.players {
		r.players[i].Wrong = 0
	}

	r.seq++
	seq := r.seq
	time.AfterFunc(time.Until(r.deadline)+50*time.Millisecond, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.seq != seq || r.phase != PhaseRound {
			return
		}
		r.timeOutLocked()
	})
	r.lastActive = time.Now()
	r.publishLocked()
}

// timeOutLocked ends a round nobody solved. It still shows the answer, because
// a round that silently restarts teaches the players nothing.
func (r *Room) timeOutLocked() {
	r.reveal = r.puzzle.Answer
	r.roundWin = ""
	r.phase = PhaseReveal
	r.seq++
	r.scheduleAdvanceLocked()
	r.publishLocked()
}

// scheduleAdvanceLocked arms the reveal pause that leads to the next round.
func (r *Room) scheduleAdvanceLocked() {
	seq := r.seq
	time.AfterFunc(r.revealPause, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.seq != seq || r.phase != PhaseReveal {
			return
		}
		r.advanceLocked()
	})
}

func (r *Room) advanceLocked() {
	if r.solved >= r.target {
		r.finishLocked()
		return
	}
	r.startRoundLocked()
}

func (r *Room) finishLocked() {
	r.phase = PhaseFinished
	r.puzzle = nil
	r.reveal = ""
	r.roundWin = ""
	r.winner = ""
	best := -1
	for _, player := range r.players {
		if player.Score > best {
			best = player.Score
			r.winner = player.ID
		} else if player.Score == best {
			// A tie has no single winner; leaving the id unset says so.
			r.winner = ""
		}
	}
	r.lastActive = time.Now()
	r.publishLocked()
}

func (r *Room) snapshotLocked() Snapshot {
	snapshot := Snapshot{
		Code:        r.code,
		Phase:       r.phase,
		HostID:      r.hostID,
		Players:     append([]Player(nil), r.players...),
		Settings:    r.settings,
		Target:      r.target,
		Solved:      r.solved,
		Round:       r.round,
		Reveal:      r.reveal,
		RoundWinner: r.roundWin,
		Winner:      r.winner,
	}
	if r.phase == PhaseRound && r.puzzle != nil {
		secondsLeft := int(time.Until(r.deadline).Seconds())
		if secondsLeft < 0 {
			secondsLeft = 0
		}
		snapshot.Puzzle = &PuzzleView{
			Scrambled:   r.scrambled,
			Category:    r.puzzle.Category,
			Length:      len(r.puzzle.Answer),
			SecondsLeft: secondsLeft,
		}
	}
	return snapshot
}

func (r *Room) publishLocked() {
	snapshot := r.snapshotLocked()
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	for _, ch := range r.subs {
		select {
		case ch <- payload:
		default:
			// Slow reader: drop this frame. The next broadcast supersedes it.
		}
	}
}

// --- helpers -----------------------------------------------------------------

// normalizeGuess reduces a guess to comparable letters: trimming, uppercasing
// and dropping anything that is not A–Z. That makes "Kucing", " kucing " and
// "KUCING!" the same answer without loosening the comparison itself.
func normalizeGuess(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range strings.ToUpper(strings.TrimSpace(raw)) {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func cleanName(raw string) string {
	name := strings.TrimSpace(raw)
	// Names are shown as text; control characters would break the scoreboard.
	name = strings.Join(strings.Fields(name), " ")
	if len(name) > 20 {
		name = name[:20]
	}
	return name
}

// pointsFor rewards length, because length is the only difficulty knob the
// players can see before committing to a guess.
func pointsFor(length int) int {
	switch {
	case length <= 4:
		return 1
	case length <= 6:
		return 2
	case length <= 8:
		return 3
	default:
		return 4
	}
}

// codeAlphabet omits characters that are misread when a code is spoken or
// copied by hand (0/O, 1/I/L).
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ"

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is not something to paper over with a guessable id.
		panic("game: entropy source unavailable: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

func newCode() string {
	code := make([]byte, 4)
	max := big.NewInt(int64(len(codeAlphabet)))
	for i := range code {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic("game: entropy source unavailable: " + err.Error())
		}
		code[i] = codeAlphabet[n.Int64()]
	}
	return string(code)
}
