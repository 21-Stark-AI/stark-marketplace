package build

import (
	"strings"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/load"
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
