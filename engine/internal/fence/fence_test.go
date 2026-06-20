package fence

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestStripKeepsMatching(t *testing.T) {
	body := "base\n<!-- runtime: claude -->\nC\n<!-- /runtime -->\n<!-- runtime: gemini -->\nG\n<!-- /runtime -->\n"
	got, err := Strip(body, model.RuntimeClaude, []model.Runtime{model.RuntimeClaude, model.RuntimeGemini})
	if err != nil {
		t.Fatal(err)
	}
	want := "base\nC\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStripExceptForm(t *testing.T) {
	body := "x\n<!-- runtime: !claude -->\nNOTCLAUDE\n<!-- /runtime -->\n"
	got, _ := Strip(body, model.RuntimeGemini, model.AllRuntimes())
	if got != "x\nNOTCLAUDE\n" {
		t.Fatalf("except form failed: %q", got)
	}
	got2, _ := Strip(body, model.RuntimeClaude, model.AllRuntimes())
	if got2 != "x\n" {
		t.Fatalf("except form should exclude claude: %q", got2)
	}
}

func TestStripErrors(t *testing.T) {
	cases := []string{
		"<!-- runtime: claude -->\nunterminated\n",                                                 // unterminated
		"<!-- runtime: claude -->\n<!-- runtime: gemini -->\n<!-- /runtime -->\n<!-- /runtime -->", // nested
		"<!-- runtime: bogus -->\nx\n<!-- /runtime -->\n",                                          // unknown runtime
	}
	for i, c := range cases {
		if _, err := Strip(c, model.RuntimeClaude, model.AllRuntimes()); err == nil {
			t.Fatalf("case %d: expected error", i)
		}
	}
}

func TestStripErrorsFenceInsideCodeBlock(t *testing.T) {
	// A runtime marker inside a ``` code block must be a validation error (spec §4.2),
	// not silently treated as a real fence.
	body := "```\n<!-- runtime: claude -->\nx\n<!-- /runtime -->\n```\n"
	if _, err := Strip(body, model.RuntimeClaude, model.AllRuntimes()); err == nil {
		t.Fatal("expected error for runtime fence inside a fenced code block")
	}
}

func TestStripErrorsEmptySection(t *testing.T) {
	// An authored fence section with no content is an error (spec §4.2).
	body := "<!-- runtime: claude -->\n\n<!-- /runtime -->\n"
	if _, err := Strip(body, model.RuntimeClaude, model.AllRuntimes()); err == nil {
		t.Fatal("expected error for empty runtime fence section")
	}
}

func TestStripKeepsCodeBlockOutsideFences(t *testing.T) {
	// A code block with no runtime markers passes through untouched.
	body := "intro\n```\ncode line\n```\ntail\n"
	got, err := Strip(body, model.RuntimeClaude, model.AllRuntimes())
	if err != nil {
		t.Fatal(err)
	}
	if got != body {
		t.Fatalf("code block mangled: got %q want %q", got, body)
	}
}
