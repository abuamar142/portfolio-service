package game

import (
	"math/rand/v2"
	"sort"
	"strings"
)

// Entry is one curated answer. Category is the hint shown to players, so it has
// to be true of the word, not decorative.
type Entry struct {
	Word     string
	Category string
}

// Word is one anagram group: the canonical answer plus every other bank word
// made of the same letters.
//
// Aggregating matters because anagram twins are real (EAT and TEA). If the
// server accepted only the canonical spelling, a player typing the twin would
// be told "wrong" for a valid word, which reads as a broken game.
type Word struct {
	Answer   string
	Category string
	Accept   []string
	Key      string
}

// Bank is the whole word corpus, grouped per language.
type Bank struct {
	byLang map[string][]Word
}

// NewBank groups entries into anagram sets.
//
// A group whose members fall into *different* categories is dropped entirely.
// The hint would otherwise be a lie: showing "food" for letters that also spell
// a verb means a player can win with a word the hint never described. Dropping
// is the only self-consistent option — keeping both members with one category
// silently misleads, and showing both categories gives the word away.
func NewBank(data map[string][]Entry) *Bank {
	bank := &Bank{byLang: make(map[string][]Word, len(data))}

	for lang, entries := range data {
		groups := make(map[string][]Entry)
		order := make([]string, 0, len(entries))
		for _, entry := range entries {
			word := normalizeGuess(entry.Word)
			// Too short to scramble meaningfully, too long to read at a glance.
			if len(word) < 3 || len(word) > 12 {
				continue
			}
			key := anagramKey(word)
			if _, seen := groups[key]; !seen {
				order = append(order, key)
			}
			groups[key] = append(groups[key], Entry{Word: word, Category: entry.Category})
		}

		words := make([]Word, 0, len(groups))
		for _, key := range order {
			members := groups[key]
			category := members[0].Category
			consistent := true
			accept := make([]string, 0, len(members))
			for _, member := range members {
				if member.Category != category {
					consistent = false
					break
				}
				accept = append(accept, member.Word)
			}
			if !consistent {
				continue
			}
			sort.Strings(accept)
			words = append(words, Word{
				Answer:   accept[0],
				Category: category,
				Accept:   accept,
				Key:      key,
			})
		}
		sort.Slice(words, func(i, j int) bool { return words[i].Key < words[j].Key })
		bank.byLang[lang] = words
	}

	return bank
}

// Len reports how many playable anagram groups a language has.
func (b *Bank) Len(lang string) int { return len(b.byLang[lang]) }

// Languages lists the languages the bank can serve.
func (b *Bank) Languages() []string {
	langs := make([]string, 0, len(b.byLang))
	for lang := range b.byLang {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return langs
}

// Pick returns a random word not yet in `used`. ok=false means the bank is
// exhausted, which ends the game rather than repeating a solved word.
func (b *Bank) Pick(lang string, used map[string]bool, rng *rand.Rand) (Word, bool) {
	pool := b.byLang[lang]
	if len(pool) == 0 {
		return Word{}, false
	}

	// Draw from the unused remainder instead of rejection-sampling: with a
	// shrinking pool, sampling would spin longer the closer the game gets to
	// the end.
	free := make([]Word, 0, len(pool))
	for _, word := range pool {
		if !used[word.Key] {
			free = append(free, word)
		}
	}
	if len(free) == 0 {
		return Word{}, false
	}
	return free[rng.IntN(len(free))], true
}

// anagramKey is the sorted letters — equal keys mean equal letter multisets.
func anagramKey(word string) string {
	letters := strings.Split(word, "")
	sort.Strings(letters)
	return strings.Join(letters, "")
}

// Scramble shuffles the letters so they no longer spell the answer.
//
// Returning the word unchanged would hand out the answer; with short words the
// shuffle hits the original often enough to matter, so it retries and then
// falls back to a rotation, which is guaranteed different for any word with at
// least two distinct letters.
func Scramble(word string, rng *rand.Rand) string {
	letters := strings.Split(word, "")
	if len(letters) < 2 {
		return word
	}
	for range 32 {
		rng.Shuffle(len(letters), func(i, j int) { letters[i], letters[j] = letters[j], letters[i] })
		candidate := strings.Join(letters, "")
		if candidate != word {
			return candidate
		}
	}
	rotated := word[1:] + word[:1]
	if rotated != word {
		return rotated
	}
	// Every letter identical (e.g. "AAA"): no distinct arrangement exists.
	return word
}
