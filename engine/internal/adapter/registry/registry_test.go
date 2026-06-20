package registry_test

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/registry"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestAllHasEveryRuntimeWithDistinctVersions(t *testing.T) {
	r := registry.All()
	for _, rt := range model.AllRuntimes() {
		if _, ok := r[rt]; !ok {
			t.Fatalf("registry missing target for runtime %q", rt)
		}
	}
	// versions are independently namespaced (§7.7) — no two targets share one.
	seen := map[string]bool{}
	for rt, tgt := range r {
		if tgt.Runtime() != rt {
			t.Fatalf("target keyed under %q reports runtime %q", rt, tgt.Runtime())
		}
		v := tgt.Version()
		if seen[v] {
			t.Fatalf("duplicate target version %q", v)
		}
		seen[v] = true
	}
}

func TestNonClaudeExcludesClaude(t *testing.T) {
	r := registry.NonClaude()
	if _, ok := r[model.RuntimeClaude]; ok {
		t.Fatal("NonClaude must not include the claude target")
	}
	if _, ok := r[model.RuntimeCodex]; !ok {
		t.Fatal("NonClaude missing codex")
	}
	if _, ok := r[model.RuntimeGemini]; !ok {
		t.Fatal("NonClaude missing gemini")
	}
}
