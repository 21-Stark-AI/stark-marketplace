package claude

import "testing"

func TestEmitFrontmatterSortedAndLF(t *testing.T) {
	fm := map[string]any{
		"name":          "review",
		"description":   "Do a review",
		"argument-hint": "[PR]",
	}
	got := emitFrontmatter([]string{"name", "description", "argument-hint"}, fm)
	// yaml.v3 single-quotes a scalar that would otherwise parse as a flow sequence
	// (leading `[`). Single quotes are valid, readable, and deterministic — the
	// emitter delegates quoting to yaml.Marshal rather than hand-rolling it.
	want := "---\nname: review\ndescription: Do a review\nargument-hint: '[PR]'\n---\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEmitFrontmatterSkipsEmpty(t *testing.T) {
	fm := map[string]any{"name": "x", "model": ""}
	got := emitFrontmatter([]string{"name", "model"}, fm)
	if got != "---\nname: x\n---\n" {
		t.Fatalf("empty field not skipped: %q", got)
	}
}

func TestEmitFrontmatterEmitsExplicitFalseBoolean(t *testing.T) {
	// A boolean PRESENT in the resolved frontmatter must survive to output even
	// when false — blanket-omitting false would silently drop an authored
	// `disable-model-invocation: false` (CC / red-team F-Cov#9).
	fm := map[string]any{"name": "x", "disable-model-invocation": false}
	got := emitFrontmatter([]string{"name", "disable-model-invocation"}, fm)
	want := "---\nname: x\ndisable-model-invocation: false\n---\n"
	if got != want {
		t.Fatalf("explicit false boolean dropped: got %q want %q", got, want)
	}
}
