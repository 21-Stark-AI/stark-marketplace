package install

import (
	"strings"
	"testing"
)

const existingMD = `# My GEMINI.md

User-authored intro paragraph.

<!-- stark:begin rev/session -->
old session block
<!-- stark:end rev/session -->

User note at the bottom.
`

func TestSentinelReplaceInPlace(t *testing.T) {
	out, action, err := MergeSentinel([]byte(existingMD), "rev/session", "new session block\n")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "User-authored intro") || !strings.Contains(s, "User note at the bottom") {
		t.Fatalf("user content lost:\n%s", s)
	}
	if strings.Contains(s, "old session block") || !strings.Contains(s, "new session block") {
		t.Fatalf("block not replaced:\n%s", s)
	}
	if action != "replace" {
		t.Fatalf("action = %s", action)
	}
}

func TestSentinelAppendsSortedWhenAbsent(t *testing.T) {
	out, action, _ := MergeSentinel([]byte(existingMD), "rev/audit", "audit block\n")
	if action != "insert" {
		t.Fatalf("action = %s", action)
	}
	s := string(out)
	if !strings.Contains(s, "rev/audit") || !strings.Contains(s, "rev/session") {
		t.Fatalf("missing a block:\n%s", s)
	}
}

func TestSentinelRefusesUnsentineledClobber(t *testing.T) {
	bad := []byte("<!-- stark:begin rev/x -->\nno end\n")
	if _, _, err := MergeSentinel(bad, "rev/x", "y\n"); err == nil {
		t.Fatal("expected error on unterminated sentinel")
	}
}

// removeSentinel must excise exactly the managed block and preserve all surrounding user
// content (§9.2 precise removal). A mutation that no-ops this would otherwise ship undetected.
func TestRemoveSentinelExcisesBlockPreservingSurroundings(t *testing.T) {
	out := string(removeSentinel([]byte(existingMD), "rev/session"))
	if strings.Contains(out, "stark:begin rev/session") || strings.Contains(out, "old session block") ||
		strings.Contains(out, "stark:end rev/session") {
		t.Fatalf("managed block not excised:\n%s", out)
	}
	if !strings.Contains(out, "User-authored intro") || !strings.Contains(out, "User note at the bottom") {
		t.Fatalf("surrounding user content lost:\n%s", out)
	}
	// removing an absent id is a no-op
	if string(removeSentinel([]byte(existingMD), "rev/absent")) != existingMD {
		t.Fatal("removing an absent sentinel must be a no-op")
	}
}
