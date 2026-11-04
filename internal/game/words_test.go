package game

import (
	mrand "math/rand/v2"
	"sort"
	"strings"
	"testing"
)

// The bank is data, so these tests are the only thing standing between a typo
// and a round nobody can solve.

func loadBank() *Bank { return NewBank(DefaultEntries()) }

func TestDefaultBankHasBothLanguages(t *testing.T) {
	bank := loadBank()
	for _, lang := range []string{"id", "en"} {
		if got := bank.Len(lang); got < 60 {
			t.Fatalf("language %q has only %d playable groups", lang, got)
		}
	}
}

func TestEveryWordIsPlayable(t *testing.T) {
	bank := loadBank()
	for _, lang := range bank.Languages() {
		for _, word := range bank.byLang[lang] {
			if len(word.Accept) == 0 {
				t.Fatalf("%s/%s has no accepted spelling", lang, word.Answer)
			}
			for _, accepted := range word.Accept {
				if accepted != normalizeGuess(accepted) {
					t.Fatalf("%s/%s is not normalized: %q", lang, word.Answer, accepted)
				}
				if anagramKey(accepted) != word.Key {
					t.Fatalf("%s/%s is not an anagram twin of %s", lang, accepted, word.Answer)
				}
			}
			if word.Category == "" {
				t.Fatalf("%s/%s has no category hint", lang, word.Answer)
			}
		}
	}
}

// A group spanning two categories would show a hint that does not describe the
// word that wins, so NewBank drops those groups instead of guessing.
func TestGroupsNeverMixCategories(t *testing.T) {
	bank := loadBank()
	for _, lang := range bank.Languages() {
		for _, word := range bank.byLang[lang] {
			for _, accepted := range word.Accept {
				if anagramKey(accepted) != anagramKey(word.Answer) {
					t.Fatalf("%s: %s and %s are not twins", lang, accepted, word.Answer)
				}
			}
		}
	}
}

func TestMixedCategoryGroupIsDropped(t *testing.T) {
	// EAT and TEA share letters but not a category; both must disappear rather
	// than let one be shown under the other's hint.
	bank := NewBank(map[string][]Entry{
		"id": {
			{"makan", "kerja"},
			{"kaman", "benda"},
			{"buku", "benda"},
		},
	})
	if bank.Len("id") != 1 {
		t.Fatalf("expected the ambiguous group dropped, got %d groups", bank.Len("id"))
	}
	if bank.byLang["id"][0].Answer != "BUKU" {
		t.Fatalf("expected BUKU to survive, got %s", bank.byLang["id"][0].Answer)
	}
}

func TestAnagramTwinsAreBothAccepted(t *testing.T) {
	bank := NewBank(map[string][]Entry{
		"id": {
			{"suka", "kerja"},
			{"kaus", "kerja"},
		},
	})
	if bank.Len("id") != 1 {
		t.Fatalf("expected one group, got %d", bank.Len("id"))
	}
	word := bank.byLang["id"][0]
	sort.Strings(word.Accept)
	want := []string{"KAUS", "SUKA"}
	if strings.Join(word.Accept, ",") != strings.Join(want, ",") {
		t.Fatalf("accepted = %v, want %v", word.Accept, want)
	}
}

func TestShortAndLongWordsAreSkipped(t *testing.T) {
	bank := NewBank(map[string][]Entry{
		"id": {
			{"ab", "benda"},
			{"abcdefghijklm", "benda"},
			{"tiga", "benda"},
		},
	})
	if bank.Len("id") != 1 {
		t.Fatalf("expected only the 4-letter word, got %d", bank.Len("id"))
	}
}

func TestPickNeverRepeatsUntilExhausted(t *testing.T) {
	bank := NewBank(map[string][]Entry{
		"id": {{"buku", "benda"}, {"meja", "benda"}, {"kursi", "benda"}},
	})
	rng := mrand.New(mrand.NewPCG(1, 2))
	used := map[string]bool{}

	for i := range 3 {
		word, ok := bank.Pick("id", used, rng)
		if !ok {
			t.Fatalf("pick %d failed before the bank was exhausted", i)
		}
		if used[word.Key] {
			t.Fatalf("pick %d repeated %s", i, word.Answer)
		}
		used[word.Key] = true
	}
	if _, ok := bank.Pick("id", used, rng); ok {
		t.Fatal("expected exhaustion after every group was used")
	}
}

func TestPickedWordMatchesAcceptList(t *testing.T) {
	bank := loadBank()
	rng := mrand.New(mrand.NewPCG(3, 4))
	for range 200 {
		word, ok := bank.Pick("id", map[string]bool{}, rng)
		if !ok {
			t.Fatal("unexpected exhaustion")
		}
		found := false
		for _, accepted := range word.Accept {
			if accepted == word.Answer {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s is not in its own accept list %v", word.Answer, word.Accept)
		}
	}
}

func TestScrambleKeepsLettersAndChangesOrder(t *testing.T) {
	rng := mrand.New(mrand.NewPCG(5, 6))
	for _, word := range []string{"HIDUP", "MEJA", "BUKU", "ELEPHANT"} {
		for range 50 {
			scrambled := Scramble(word, rng)
			if scrambled == word {
				t.Fatalf("%s was not scrambled", word)
			}
			if anagramKey(scrambled) != anagramKey(word) {
				t.Fatalf("%s scrambled to %s (different letters)", word, scrambled)
			}
		}
	}
}

func TestScrambleHandlesRepeatedLetters(t *testing.T) {
	rng := mrand.New(mrand.NewPCG(7, 8))
	// AAA has exactly one arrangement; returning it is correct, not a bug.
	if got := Scramble("AAA", rng); got != "AAA" {
		t.Fatalf("Scramble(AAA) = %s, want AAA", got)
	}
	if got := Scramble("KK", rng); got != "KK" {
		t.Fatalf("Scramble(KK) = %s, want KK", got)
	}
}

func TestNormalizeGuess(t *testing.T) {
	cases := map[string]string{
		"  kucing ": "KUCING",
		"KuCiNg!":   "KUCING",
		"ma-kan":    "MAKAN",
		"é":         "",
		"":          "",
	}
	for input, want := range cases {
		if got := normalizeGuess(input); got != want {
			t.Errorf("normalizeGuess(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPointsFor(t *testing.T) {
	cases := []struct{ length, want int }{
		{3, 1}, {4, 1}, {5, 2}, {6, 2}, {7, 3}, {8, 3}, {9, 4}, {12, 4},
	}
	for _, c := range cases {
		if got := pointsFor(c.length); got != c.want {
			t.Errorf("pointsFor(%d) = %d, want %d", c.length, got, c.want)
		}
	}
}

func TestTargetFor(t *testing.T) {
	cases := []struct {
		explicit, players, want int
	}{
		{0, 1, 5}, {0, 2, 5}, {0, 3, 6}, {0, 5, 10}, {7, 9, 7}, {0, 12, 24},
	}
	for _, c := range cases {
		if got := targetFor(c.explicit, c.players); got != c.want {
			t.Errorf("targetFor(%d, %d) = %d, want %d", c.explicit, c.players, got, c.want)
		}
	}
}

func TestCodeAlphabetHasNoLookalikes(t *testing.T) {
	for _, bad := range []string{"O", "0", "I", "1", "L"} {
		if strings.Contains(codeAlphabet, bad) {
			t.Fatalf("code alphabet contains lookalike %q", bad)
		}
	}
	if got := normalizeCode("ab-cd"); got != "ABCD" {
		t.Fatalf("normalizeCode(ab-cd) = %s, want ABCD", got)
	}
}
