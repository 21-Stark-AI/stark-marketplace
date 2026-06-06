package merge

import "testing"

func TestParseDivergedReason(t *testing.T) {
	body := "# diverged: gemini needs a different opening\nReal body line\n"
	reason, stripped := parseDivergedReason(body)
	if reason != "gemini needs a different opening" {
		t.Fatalf("reason = %q", reason)
	}
	if stripped != "Real body line\n" {
		t.Fatalf("stripped = %q", stripped)
	}
}

func TestParseDivergedReasonAbsent(t *testing.T) {
	reason, stripped := parseDivergedReason("Just a body\n")
	if reason != "" {
		t.Fatalf("expected no reason, got %q", reason)
	}
	if stripped != "Just a body\n" {
		t.Fatalf("stripped = %q", stripped)
	}
}

func TestArrayPrefixMismatch(t *testing.T) {
	// override array drops a base element that is NOT a trailing addition -> warn
	if !arrayDropsPrefix([]any{"a", "b", "c"}, []any{"a", "c"}) {
		t.Fatal("expected mismatch (b dropped mid-prefix)")
	}
	// override is a superset-by-prefix -> no warning
	if arrayDropsPrefix([]any{"a", "b"}, []any{"a", "b", "c"}) {
		t.Fatal("superset-by-prefix should not warn")
	}
	// identical -> no warning
	if arrayDropsPrefix([]any{"a"}, []any{"a"}) {
		t.Fatal("identical should not warn")
	}
}
