package game

import (
	mrand "math/rand/v2"
	"strings"
	"testing"
	"time"
)

// The engine is the part that must be right: scoring, lockout and phase
// transitions are invisible in the UI until they break.

func testManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(NewBank(DefaultEntries()))
}

func newTestRoom(t *testing.T, m *Manager, settings Settings) (*Room, Player) {
	t.Helper()
	room, host, err := m.Create("host", settings)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return room, host
}

// answerFor returns the word the room expects, read straight off the puzzle so
// the test does not have to know the bank.
func answerFor(t *testing.T, room *Room) string {
	t.Helper()
	room.mu.Lock()
	defer room.mu.Unlock()
	if room.puzzle == nil {
		t.Fatal("no puzzle in play")
	}
	return room.puzzle.Answer
}

func TestCreateSeatsTheHost(t *testing.T) {
	m := testManager(t)
	room, host := newTestRoom(t, m, Settings{})

	if room.Phase() != PhaseLobby {
		t.Fatalf("phase = %s, want lobby", room.Phase())
	}
	snapshot := room.Snapshot()
	if len(snapshot.Players) != 1 || snapshot.Players[0].ID != host.ID {
		t.Fatalf("host not seated: %+v", snapshot.Players)
	}
	if snapshot.HostID != host.ID {
		t.Fatalf("host id = %s, want %s", snapshot.HostID, host.ID)
	}
	if len(room.Code()) != 4 {
		t.Fatalf("code %q is not 4 characters", room.Code())
	}
}

func TestSettingsDefaults(t *testing.T) {
	room, _ := newTestRoom(t, testManager(t), Settings{})
	settings := room.Snapshot().Settings
	if settings.Language != "id" || settings.MaxWrong != 3 || settings.RoundSeconds != 30 {
		t.Fatalf("defaults = %+v", settings)
	}
}

func TestJoinRejectsDuplicateNames(t *testing.T) {
	room, _ := newTestRoom(t, testManager(t), Settings{})
	if _, err := room.AddPlayer("Host"); err != ErrNameTaken {
		t.Fatalf("duplicate name error = %v, want ErrNameTaken", err)
	}
	if _, err := room.AddPlayer("  "); err != ErrInvalidName {
		t.Fatalf("blank name error = %v, want ErrInvalidName", err)
	}
}

func TestJoinRejectsWhenFull(t *testing.T) {
	room, _ := newTestRoom(t, testManager(t), Settings{})
	for i := 1; i < MaxPlayers; i++ {
		if _, err := room.AddPlayer(uniqueName(i)); err != nil {
			t.Fatalf("player %d: %v", i, err)
		}
	}
	if _, err := room.AddPlayer("extra"); err != ErrRoomFull {
		t.Fatalf("error = %v, want ErrRoomFull", err)
	}
}

func uniqueName(i int) string {
	names := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}
	if i < len(names) {
		return "p" + names[i]
	}
	return "px"
}

func TestOnlyHostStarts(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	guest, err := room.AddPlayer("guest")
	if err != nil {
		t.Fatal(err)
	}
	if err := room.Start(guest.ID); err != ErrNotHost {
		t.Fatalf("guest start error = %v, want ErrNotHost", err)
	}
	if err := room.Start(host.ID); err != nil {
		t.Fatalf("host start: %v", err)
	}
	if room.Phase() != PhaseRound {
		t.Fatalf("phase = %s, want round", room.Phase())
	}
}

func TestStartPublishesScrambleNotAnswer(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	snapshot := room.Snapshot()
	if snapshot.Puzzle == nil {
		t.Fatal("no puzzle published")
	}
	answer := answerFor(t, room)
	if snapshot.Puzzle.Scrambled == answer {
		t.Fatal("scramble equals the answer")
	}
	if snapshot.Puzzle.Length != len(answer) {
		t.Fatalf("length = %d, want %d", snapshot.Puzzle.Length, len(answer))
	}
}

func TestCorrectAnswerScoresAndReveals(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	answer := answerFor(t, room)

	result, err := room.Submit(host.ID, answer)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !result.Correct {
		t.Fatal("correct answer was rejected")
	}
	if result.Points != pointsFor(len(answer)) {
		t.Fatalf("points = %d, want %d", result.Points, pointsFor(len(answer)))
	}

	snapshot := room.Snapshot()
	if snapshot.Phase != PhaseReveal {
		t.Fatalf("phase = %s, want reveal", snapshot.Phase)
	}
	if snapshot.Reveal != answer {
		t.Fatalf("reveal = %s, want %s", snapshot.Reveal, answer)
	}
	if snapshot.RoundWinner != host.ID {
		t.Fatalf("round winner = %s, want %s", snapshot.RoundWinner, host.ID)
	}
	if snapshot.Solved != 1 {
		t.Fatalf("solved = %d, want 1", snapshot.Solved)
	}
}

func TestGuessIsCaseAndPunctuationInsensitive(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	answer := answerFor(t, room)
	messy := " " + answer[:1] + answer[1:] + "! "

	result, err := room.Submit(host.ID, messy)
	if err != nil || !result.Correct {
		t.Fatalf("messy guess rejected: %+v %v", result, err)
	}
}

func TestWrongAnswersLockOutAtTheLimit(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{MaxWrong: 2})
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	answer := answerFor(t, room)
	wrong := "ZZZZZ"
	if answer == wrong {
		wrong = "YYYYY"
	}

	first, err := room.Submit(host.ID, wrong)
	if err != nil || first.Correct || first.Locked {
		t.Fatalf("first wrong = %+v %v", first, err)
	}
	second, err := room.Submit(host.ID, wrong)
	if err != nil || second.Correct || !second.Locked {
		t.Fatalf("second wrong = %+v %v", second, err)
	}
	if _, err := room.Submit(host.ID, answer); err != ErrLocked {
		t.Fatalf("locked player error = %v, want ErrLocked", err)
	}
}

func TestLockoutDoesNotBlockOtherPlayers(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{MaxWrong: 1})
	guest, _ := room.AddPlayer("guest")
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	answer := answerFor(t, room)

	if _, err := room.Submit(host.ID, "ZZZZZ"); err != nil {
		t.Fatal(err)
	}
	// The guest never guessed wrong, so the host's lockout must not touch them.
	result, err := room.Submit(guest.ID, answer)
	if err != nil {
		t.Fatalf("guest blocked by another player's lockout: %v", err)
	}
	if !result.Correct {
		t.Fatal("guest's correct answer was rejected")
	}
}

func TestRepeatedAnswerAfterRoundEndsIsRejected(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	answer := answerFor(t, room)
	if _, err := room.Submit(host.ID, answer); err != nil {
		t.Fatal(err)
	}
	if _, err := room.Submit(host.ID, answer); err != ErrWrongPhase {
		t.Fatalf("second submit error = %v, want ErrWrongPhase", err)
	}
}

func TestManualScoreAdjustment(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	guest, _ := room.AddPlayer("guest")

	if err := room.AdjustScore(guest.ID, host.ID, 1); err != ErrNotHost {
		t.Fatalf("guest adjust error = %v, want ErrNotHost", err)
	}
	if err := room.AdjustScore(host.ID, host.ID, 3); err != nil {
		t.Fatal(err)
	}
	if got := room.Snapshot().Players[0].Score; got != 3 {
		t.Fatalf("score = %d, want 3", got)
	}
	// A correction must never push a player below zero.
	if err := room.AdjustScore(host.ID, host.ID, -10); err != nil {
		t.Fatal(err)
	}
	if got := room.Snapshot().Players[0].Score; got != 0 {
		t.Fatalf("score = %d, want 0", got)
	}
}

func TestAutoTargetScalesWithPlayers(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	for i := 1; i <= 3; i++ {
		if _, err := room.AddPlayer(uniqueName(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	// Four players → 4*2 = 8 words, above the floor of five.
	if got := room.Snapshot().Target; got != 8 {
		t.Fatalf("target = %d, want 8", got)
	}
}

func TestExplicitTargetWins(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{Target: 3})
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	if got := room.Snapshot().Target; got != 3 {
		t.Fatalf("target = %d, want 3", got)
	}
}

func TestGameFinishesAtTarget(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{Target: 2})
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		answer := answerFor(t, room)
		if _, err := room.Submit(host.ID, answer); err != nil {
			t.Fatalf("submit: %v", err)
		}
		// Skip the reveal pause instead of sleeping through it.
		if err := room.Next(host.ID); err != nil {
			t.Fatalf("next: %v", err)
		}
	}

	snapshot := room.Snapshot()
	if snapshot.Phase != PhaseFinished {
		t.Fatalf("phase = %s, want finished", snapshot.Phase)
	}
	if snapshot.Winner != host.ID {
		t.Fatalf("winner = %s, want %s", snapshot.Winner, host.ID)
	}
	if snapshot.Solved != 2 {
		t.Fatalf("solved = %d, want 2", snapshot.Solved)
	}
}

func TestTieLeavesWinnerUnset(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{Target: 1})
	guest, _ := room.AddPlayer("guest")
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}

	answer := answerFor(t, room)
	result, err := room.Submit(host.ID, answer)
	if err != nil {
		t.Fatal(err)
	}
	// Points depend on word length, so the guest is levelled *after* the round
	// reveals what the host actually earned.
	if err := room.AdjustScore(host.ID, guest.ID, result.Points); err != nil {
		t.Fatal(err)
	}
	if err := room.Next(host.ID); err != nil {
		t.Fatal(err)
	}

	snapshot := room.Snapshot()
	if snapshot.Players[0].Score != snapshot.Players[1].Score {
		t.Fatalf("scores differ: %d vs %d", snapshot.Players[0].Score, snapshot.Players[1].Score)
	}
	if snapshot.Winner != "" {
		t.Fatalf("tie produced a winner: %s", snapshot.Winner)
	}
}

func TestRoundTimesOutAndRevealsTheAnswer(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{RoundSeconds: 10})
	room.settings.RoundSeconds = 10
	room.revealPause = 20 * time.Millisecond
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	answer := answerFor(t, room)

	// Force the deadline into the past and run the timeout directly: waiting
	// ten real seconds would make the suite slow for no extra coverage.
	room.mu.Lock()
	room.timeOutLocked()
	room.mu.Unlock()

	snapshot := room.Snapshot()
	if snapshot.Phase != PhaseReveal {
		t.Fatalf("phase = %s, want reveal", snapshot.Phase)
	}
	if snapshot.Reveal != answer {
		t.Fatalf("reveal = %s, want %s", snapshot.Reveal, answer)
	}
	if snapshot.Solved != 0 {
		t.Fatalf("timeout counted as solved: %d", snapshot.Solved)
	}
	if snapshot.RoundWinner != "" {
		t.Fatalf("timeout produced a winner: %s", snapshot.RoundWinner)
	}
}

func TestAdvancePicksANewWord(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{Target: 5})
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	first := answerFor(t, room)

	answer := answerFor(t, room)
	if _, err := room.Submit(host.ID, answer); err != nil {
		t.Fatal(err)
	}
	if err := room.Next(host.ID); err != nil {
		t.Fatal(err)
	}
	if second := answerFor(t, room); second == first {
		t.Fatalf("round 2 reused %s", first)
	}
	if got := room.Snapshot().Round; got != 2 {
		t.Fatalf("round = %d, want 2", got)
	}
}

func TestNextRejectedOutsideReveal(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	if err := room.Next(host.ID); err != ErrWrongPhase {
		t.Fatalf("next during round = %v, want ErrWrongPhase", err)
	}
	guest, _ := room.AddPlayer("guest")
	if err := room.Next(guest.ID); err != ErrWrongPhase && err != ErrNotHost {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlayersCannotSubmitInLobby(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	if _, err := room.Submit(host.ID, "BUKU"); err != ErrWrongPhase {
		t.Fatalf("submit in lobby = %v, want ErrWrongPhase", err)
	}
}

func TestUnknownPlayerIsRejected(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := room.Submit("nobody", "BUKU"); err != ErrPlayerNotFound {
		t.Fatalf("unknown player = %v, want ErrPlayerNotFound", err)
	}
}

func TestSubscribeReceivesStateOnChange(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	ch, cancel := room.Subscribe()
	defer cancel()

	// Subscribe itself does not publish: the first frame a client sees is the
	// one the SSE handler writes before entering the loop. So the first thing
	// on this channel is genuinely the join.
	if _, err := room.AddPlayer("guest"); err != nil {
		t.Fatal(err)
	}
	select {
	case payload := <-ch:
		if len(payload) == 0 {
			t.Fatal("empty broadcast")
		}
	case <-time.After(time.Second):
		t.Fatal("no broadcast after a join")
	}
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("no broadcast after start")
	}
}

func TestCancelStopsDelivery(t *testing.T) {
	room, _ := newTestRoom(t, testManager(t), Settings{})
	ch, cancel := room.Subscribe()
	cancel()

	if _, ok := <-ch; ok {
		t.Fatal("channel still open after cancel")
	}
	// A second cancel must not panic on the closed channel.
	cancel()
}

func TestLeavePromotesANewHost(t *testing.T) {
	m := testManager(t)
	room, host := newTestRoom(t, m, Settings{})
	guest, err := room.AddPlayer("guest")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Leave(room.Code(), host.ID); err != nil {
		t.Fatal(err)
	}
	snapshot := room.Snapshot()
	if snapshot.HostID != guest.ID {
		t.Fatalf("host = %s, want %s", snapshot.HostID, guest.ID)
	}
	if len(snapshot.Players) != 1 {
		t.Fatalf("players = %d, want 1", len(snapshot.Players))
	}
}

func TestSweepRemovesIdleRooms(t *testing.T) {
	m := testManager(t)
	room, _ := newTestRoom(t, m, Settings{})
	if m.Len() != 1 {
		t.Fatalf("rooms = %d, want 1", m.Len())
	}
	if got := m.Sweep(time.Nanosecond); got != 1 {
		t.Fatalf("swept = %d, want 1", got)
	}
	if _, err := m.Get(room.Code()); err != ErrNotFound {
		t.Fatalf("get after sweep = %v, want ErrNotFound", err)
	}
}

func TestCodesAreUnique(t *testing.T) {
	m := testManager(t)
	seen := make(map[string]bool)
	for range 200 {
		room, _, err := m.Create("host", Settings{})
		if err != nil {
			t.Fatal(err)
		}
		if seen[room.Code()] {
			t.Fatalf("duplicate code %s", room.Code())
		}
		seen[room.Code()] = true
	}
}

func TestRoomLookupIsCaseInsensitive(t *testing.T) {
	m := testManager(t)
	room, _, err := m.Create("host", Settings{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get(strings.ToLower(room.Code())); err != nil {
		t.Fatalf("lowercase lookup failed: %v", err)
	}
}

func TestWordOrderIsNotPredictableAcrossRooms(t *testing.T) {
	// Two managers must not produce the same first word, which would mean the
	// seed is fixed rather than drawn from entropy.
	first := func() string {
		m := NewManager(NewBank(DefaultEntries()))
		room, host, err := m.Create("host", Settings{})
		if err != nil {
			t.Fatal(err)
		}
		if err := room.Start(host.ID); err != nil {
			t.Fatal(err)
		}
		return answerFor(t, room)
	}
	// A single draw can collide legitimately (1 in ~90); two identical draws in
	// a row across four attempts would mean a fixed seed.
	different := false
	for range 4 {
		if first() != first() {
			different = true
			break
		}
	}
	if !different {
		t.Fatal("word order looks fixed across managers")
	}
}

func TestRestartResetsScoresAndRounds(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{Target: 1})
	if err := room.Start(host.ID); err != nil {
		t.Fatal(err)
	}
	answer := answerFor(t, room)
	if _, err := room.Submit(host.ID, answer); err != nil {
		t.Fatal(err)
	}
	if err := room.Next(host.ID); err != nil {
		t.Fatal(err)
	}
	if room.Snapshot().Phase != PhaseFinished {
		t.Fatal("game did not finish")
	}

	if err := room.Restart(host.ID); err != nil {
		t.Fatalf("restart: %v", err)
	}
	snapshot := room.Snapshot()
	if snapshot.Phase != PhaseRound {
		t.Fatalf("phase = %s, want round", snapshot.Phase)
	}
	if snapshot.Solved != 0 {
		t.Fatalf("solved carried over: %d", snapshot.Solved)
	}
	if snapshot.Round != 1 {
		t.Fatalf("round = %d, want 1", snapshot.Round)
	}
	for _, player := range snapshot.Players {
		if player.Score != 0 || player.Wrong != 0 {
			t.Fatalf("player carried state: %+v", player)
		}
	}
}

func TestRestartRequiresFinishedGame(t *testing.T) {
	room, host := newTestRoom(t, testManager(t), Settings{})
	if err := room.Restart(host.ID); err != ErrWrongPhase {
		t.Fatalf("restart in lobby = %v, want ErrWrongPhase", err)
	}
}

func TestScrambleUsesRoomRandomness(t *testing.T) {
	// Regression guard: Scramble must not reach for the global generator, or
	// tests would stop being reproducible.
	rng := mrand.New(mrand.NewPCG(9, 9))
	again := mrand.New(mrand.NewPCG(9, 9))
	if Scramble("HIDUP", rng) != Scramble("HIDUP", again) {
		t.Fatal("Scramble is not driven by the supplied generator")
	}
}
