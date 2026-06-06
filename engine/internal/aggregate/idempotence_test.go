package aggregate

import (
	"math/rand"
	"testing"
)

func TestMergeIsIdempotentViaParse(t *testing.T) {
	// Property: Merge(Parse(Merge(S))) == Merge(S) for any section set S,
	// regardless of input order.
	rng := rand.New(rand.NewSource(1))
	for iter := 0; iter < 200; iter++ {
		n := rng.Intn(6)
		secs := make([]Section, n)
		for i := range secs {
			secs[i] = Section{
				Bundle:  string(rune('a' + rng.Intn(4))),
				Name:    string(rune('a' + rng.Intn(4))),
				Content: "body-" + string(rune('a'+rng.Intn(3))) + "\n",
			}
		}
		// dedupe by id (real catalogs have unique ids); keep last writer.
		seen := map[string]Section{}
		var uniq []Section
		for _, s := range secs {
			seen[s.id()] = s
		}
		for _, s := range seen {
			uniq = append(uniq, s)
		}

		first := Merge(uniq)
		reparsed := Parse(first)
		again := Merge(reparsed)
		if first != again {
			t.Fatalf("not idempotent (iter %d):\n--- first ---\n%s\n--- again ---\n%s", iter, first, again)
		}
	}
}

func TestParseRoundTripsSections(t *testing.T) {
	in := []Section{
		{Bundle: "a", Name: "x", Content: "AX\n"},
		{Bundle: "b", Name: "y", Content: "BY\n"},
	}
	parsed := Parse(Merge(in))
	if len(parsed) != 2 {
		t.Fatalf("want 2 sections, got %d", len(parsed))
	}
}
