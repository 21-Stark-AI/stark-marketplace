package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/load"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

var update = os.Getenv("UPDATE_GOLDEN") == "1"

// TestGoldenSeedBundle renders a FROZEN fixture bundle (testdata/seed-catalog)
// end-to-end through the loader + claude adapter and byte-compares each rendered
// file to a checked-in golden. It deliberately does NOT load the live
// ../../../../catalog: coupling the golden to a live bundle meant every skill
// edit and every version bump (the marketplace-sync auto-bump) drifted this test
// and could merge the catalog red (issue #38). The live catalog's rendering is
// still gated by `stark sync --check` / `build --check` in CI; this test guards
// the adapter's output FORMAT (plugin.json, .mcp.json, skills/, commands/) against
// a fixture that only changes when someone deliberately edits it. Run with
// UPDATE_GOLDEN=1 to regenerate after an intentional format change.
func TestGoldenSeedBundle(t *testing.T) {
	cat, err := load.Load(filepath.Join("testdata", "seed-catalog"))
	if err != nil {
		t.Fatal(err)
	}
	bundle := cat.Bundles[0] // sole fixture bundle: seed-fixture
	files, _, err := New().Render(bundle)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("testdata", "golden", bundle.Name)
	assertGoldenTree(t, dir, files)
}

// TestGoldenSyntheticTypes locks byte-exact output for skill, agent, and prompt —
// types the seed catalog doesn't exercise — so frontmatter field ordering and body
// composition for those paths are guarded, not just substring-checked.
func TestGoldenSyntheticTypes(t *testing.T) {
	b := &model.Bundle{Name: "synthetic", Runtimes: []model.Runtime{model.RuntimeClaude}, Artifacts: []*model.Artifact{
		{Name: "rev", Type: model.TypeSkill, Description: "review skill", Version: "0.1.0",
			DisableModelInvocation: true, AllowedTools: []string{"Bash", "Read"}, Model: "opus",
			Runtimes: []model.Runtime{model.RuntimeClaude}, Raw: map[string]any{}, Body: "Skill body.\n"},
		{Name: "rt", Type: model.TypeAgent, Description: "red team", Version: "0.1.0",
			Tools: []string{"Bash"}, Model: "opus",
			Runtimes: []model.Runtime{model.RuntimeClaude}, Raw: map[string]any{}, Body: "Agent body.\n"},
		{Name: "ask", Type: model.TypePrompt, Description: "a prompt", Version: "0.1.0",
			Runtimes: []model.Runtime{model.RuntimeClaude}, Raw: map[string]any{}, Body: "Prompt body.\n"},
	}}
	files, _, err := New().Render(b)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("testdata", "golden-synthetic")
	assertGoldenTree(t, dir, files)
}

// assertGoldenTree compares rendered files to checked-in goldens under dir AND
// fails on orphan goldens (a golden with no corresponding rendered file) so a
// dropped output path can't silently pass. UPDATE_GOLDEN=1 rewrites + prunes.
func assertGoldenTree(t *testing.T, dir string, files []adapter.OutputFile) {
	t.Helper()
	rendered := map[string]bool{}
	for _, f := range files {
		rendered[f.Path] = true
		gp := filepath.Join(dir, filepath.FromSlash(f.Path))
		if update {
			_ = os.MkdirAll(filepath.Dir(gp), 0o755)
			if err := os.WriteFile(gp, f.Content, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(gp)
		if err != nil {
			t.Fatalf("missing golden %s (run UPDATE_GOLDEN=1): %v", gp, err)
		}
		if string(want) != string(f.Content) {
			t.Fatalf("golden mismatch %s:\n--- want ---\n%s\n--- got ---\n%s", f.Path, want, f.Content)
		}
	}
	// orphan detection: every checked-in golden must correspond to a rendered file.
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if rendered[rel] {
			return nil
		}
		if update {
			_ = os.Remove(path) // prune orphan during regeneration
			return nil
		}
		t.Fatalf("orphan golden %s has no corresponding rendered file", rel)
		return nil
	})
}
