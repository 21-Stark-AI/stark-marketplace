package fence

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
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
		"<!-- runtime: claude -->\nunterminated\n",                 // unterminated
		"<!-- runtime: claude -->\n<!-- runtime: gemini -->\n<!-- /runtime -->\n<!-- /runtime -->", // nested
		"<!-- runtime: bogus -->\nx\n<!-- /runtime -->\n",          // unknown runtime
	}
	for i, c := range cases {
		if _, err := Strip(c, model.RuntimeClaude, model.AllRuntimes()); err == nil {
			t.Fatalf("case %d: expected error", i)
		}
	}
}
