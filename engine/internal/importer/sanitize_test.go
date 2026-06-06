package importer

import (
	"strings"
	"testing"
)

// Real stark-skills frontmatter writes argument-hint unquoted with YAML-significant chars,
// which strict YAML rejects. decodeFrontmatter must fall back to a sanitized re-parse.
func TestDecodeFrontmatterSanitizesLooseArgumentHint(t *testing.T) {
	loose := []byte("name: rel\n" +
		"description: a release skill\n" +
		"argument-hint: [patch|minor|major] (optional — auto-detected if omitted)\n" +
		"model: sonnet\n")
	raw, sanitized, err := decodeFrontmatter(loose)
	if err != nil {
		t.Fatalf("loose frontmatter should parse via sanitize fallback: %v", err)
	}
	if !sanitized {
		t.Fatal("expected sanitized=true for the loose argument-hint")
	}
	hint, ok := raw["argument-hint"].(string)
	if !ok || hint != "[patch|minor|major] (optional — auto-detected if omitted)" {
		t.Fatalf("argument-hint not recovered as a string: %v (%T)", raw["argument-hint"], raw["argument-hint"])
	}
	if raw["name"] != "rel" || raw["model"] != "sonnet" {
		t.Fatalf("other fields lost: %+v", raw)
	}
}

// A pre-quoted argument-hint followed by an inline comment must NOT be re-quoted (the
// always-sanitize pass must leave an already-quoted scalar to YAML, comment and all).
func TestSanitizeLeavesQuotedHintWithTrailingComment(t *testing.T) {
	raw, sanitized, err := decodeFrontmatter([]byte("argument-hint: \"[a|b]\"  # pick one\n"))
	if err != nil {
		t.Fatal(err)
	}
	if sanitized {
		t.Fatal("already-quoted hint must not be reported as sanitized")
	}
	if raw["argument-hint"] != "[a|b]" {
		t.Fatalf("quoted hint corrupted: %v", raw["argument-hint"])
	}
}

func TestDecodeFrontmatterStrictPathUnchanged(t *testing.T) {
	clean := []byte("name: rel\nargument-hint: \"[patch|minor|major]\"\n")
	raw, sanitized, err := decodeFrontmatter(clean)
	if err != nil {
		t.Fatal(err)
	}
	if sanitized {
		t.Fatal("already-valid frontmatter must not report sanitized")
	}
	if raw["argument-hint"] != "[patch|minor|major]" {
		t.Fatalf("argument-hint = %v", raw["argument-hint"])
	}
}

// `argument-hint: [start|end]` is VALID YAML (a flow sequence), so strict parse succeeds and
// yields a []any — without unconditional sanitizing the .(string) carry drops it silently.
// This is the real stark-skills shape (e.g. skill/stark-session). It must become a string.
func TestDecodeFrontmatterFlowSequenceArgumentHintBecomesString(t *testing.T) {
	for _, in := range []string{
		"argument-hint: [start|end]\n",
		"argument-hint: [--dry-run]\n",
		"argument-hint: [patch|minor|major]\n",
	} {
		raw, sanitized, err := decodeFrontmatter([]byte(in))
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if !sanitized {
			t.Fatalf("%q: expected sanitized=true (loose flow-sequence hint)", in)
		}
		s, ok := raw["argument-hint"].(string)
		if !ok {
			t.Fatalf("%q: argument-hint is %T, want string", in, raw["argument-hint"])
		}
		if !strings.HasPrefix(s, "[") {
			t.Fatalf("%q: argument-hint string lost its literal text: %q", in, s)
		}
	}
}
