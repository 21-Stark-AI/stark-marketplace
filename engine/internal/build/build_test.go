package build

import (
	"strings"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/load"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestBuildProducesClaudeTreeAndIndex(t *testing.T) {
	cat, err := load.Load("../../../catalog")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Build(cat)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.Files["index.json"]; !ok {
		t.Fatal("index.json not produced")
	}
	if _, ok := out.Files["bundles/stark-gh.json"]; !ok {
		t.Fatal("bundle detail not produced")
	}
	foundClaude := false
	for p := range out.Files {
		if strings.HasPrefix(p, "dist/claude/stark-gh/") {
			foundClaude = true
		}
	}
	if !foundClaude {
		t.Fatalf("no dist/claude files; got %v", keys(out.Files))
	}
	// divergence budget present (seed has 0 diverged)
	if !strings.Contains(out.DivergenceBudget, "diverged") {
		t.Fatalf("budget = %q", out.DivergenceBudget)
	}
}

func keys(m map[string][]byte) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestBuildEmitsMarketplaceManifest(t *testing.T) {
	cat, err := load.Load("../../../catalog")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Build(cat)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := out.Files[".claude-plugin/marketplace.json"]
	if !ok {
		t.Fatal("marketplace.json not produced into the generated set (repo-root .claude-plugin)")
	}
	s := string(b)
	// root owner, entry author (red-team Part B), seed bundle present.
	if !strings.Contains(s, `"owner"`) || !strings.Contains(s, `"author"`) {
		t.Fatalf("manifest shape wrong: %s", s)
	}
	if !strings.Contains(s, `"stark-gh"`) || !strings.Contains(s, `"./dist/claude/stark-gh"`) {
		t.Fatalf("manifest missing seed bundle / source: %s", s)
	}
}

func TestDivergenceBudgetCountsDiverged(t *testing.T) {
	// A claude artifact with an annotated full-body override is author-divergence:
	// merge.Resolve marks it Diverged, claude.foldFindings prefixes "diverged: ", and
	// build.Build counts it. Locks that whole string-coupled chain at a nonzero count.
	cat := &model.Catalog{Bundles: []*model.Bundle{{
		Name: "demo", Version: "0.1.0", Description: "d", Owner: model.Owner{Name: "E"},
		Runtimes: []model.Runtime{model.RuntimeClaude},
		Artifacts: []*model.Artifact{{
			Name: "rev", Type: model.TypeSkill, Description: "s", Version: "0.1.0",
			Runtimes: []model.Runtime{model.RuntimeClaude}, Raw: map[string]any{}, Body: "base\n",
			Overrides: map[model.Runtime]model.Override{
				model.RuntimeClaude: {Body: "# diverged: needs a claude-specific body\nClaude body\n"},
			},
		}},
	}}}
	out, err := Build(cat)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.DivergenceBudget, "diverged 1 /") {
		t.Fatalf("want budget to count 1 diverged, got %q", out.DivergenceBudget)
	}
}
