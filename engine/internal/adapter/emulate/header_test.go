package emulate

import (
	"strings"
	"testing"
)

func TestHeaderContainsProvenanceAndWarning(t *testing.T) {
	h := Header("stark-review", "red-team", "<!-- ", " -->")
	if !strings.Contains(h, "EMULATED from stark-review/red-team") {
		t.Fatalf("missing provenance: %q", h)
	}
	if !strings.Contains(h, "may not auto-activate") || !strings.Contains(h, "verify") {
		t.Fatalf("missing fidelity warning: %q", h)
	}
	if !strings.HasPrefix(h, "<!-- ") {
		t.Fatalf("header must be wrapped in comment open delim: %q", h)
	}
	if !strings.HasSuffix(h, "\n") {
		t.Fatal("header must end with a newline so it precedes content cleanly")
	}
}

func TestHeaderDeterministic(t *testing.T) {
	a := Header("b", "n", "# ", "")
	b := Header("b", "n", "# ", "")
	if a != b {
		t.Fatal("header must be a pure function of its inputs (no clock/random)")
	}
}
