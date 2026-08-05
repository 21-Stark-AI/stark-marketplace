package marketplace

import (
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/model"
)

func TestGenerateCodexPluginManifest(t *testing.T) {
	b := &model.Bundle{
		Name:        "stark-gh",
		Description: "GitHub pull-request workflows for Stark.",
		Category:    "productivity",
		Tags:        []string{"github", "workflow"},
		Owner:       model.Owner{Name: "21 Stark AI", Email: "engineering@21stark.com"},
		Homepage:    "https://example.com/stark-gh",
	}

	got, err := GenerateCodexPlugin(b, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != b.Name || got.Version != "1.2.3" || got.Skills != "./skills/" {
		t.Fatalf("identity/path mismatch: %#v", got)
	}
	if got.Author.Name != b.Owner.Name || got.Author.Email != b.Owner.Email {
		t.Fatalf("author mismatch: %#v", got.Author)
	}
	if got.Interface.DisplayName != "Stark Gh" || got.Interface.Category != "Productivity" {
		t.Fatalf("interface identity mismatch: %#v", got.Interface)
	}
	if len(got.Interface.Capabilities) == 0 || len(got.Interface.DefaultPrompt) != 1 {
		t.Fatalf("required interface metadata missing: %#v", got.Interface)
	}
	b1, err := MarshalCodex(got)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := MarshalCodex(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) || !strings.HasSuffix(string(b1), "\n") {
		t.Fatal("Codex manifest encoding is not deterministic with one trailing newline")
	}
}

func TestGenerateCodexPluginRejectsInvalidVersion(t *testing.T) {
	_, err := GenerateCodexPlugin(&model.Bundle{Name: "demo"}, "dev")
	if err == nil || !strings.Contains(err.Error(), "strict semver") {
		t.Fatalf("want strict semver error, got %v", err)
	}
}

func TestGenerateCodexMarketplaceSortedAndFiltered(t *testing.T) {
	artifact := func(rt model.Runtime) *model.Artifact {
		return &model.Artifact{Name: "skill", Type: model.TypeSkill, Runtimes: []model.Runtime{rt}}
	}
	cat := &model.Catalog{Bundles: []*model.Bundle{
		{Name: "zeta", Category: "ops", Artifacts: []*model.Artifact{artifact(model.RuntimeCodex)}},
		{Name: "claude-only", Artifacts: []*model.Artifact{artifact(model.RuntimeClaude)}},
		{Name: "alpha", Artifacts: []*model.Artifact{artifact(model.RuntimeCodex)}},
	}}

	got := GenerateCodexMarketplace(cat, CodexMarketplaceOptions{
		Name: "bifrost", DisplayName: "Bifrost", DistRoot: "./dist/codex-plugins/",
	})
	if len(got.Plugins) != 2 {
		t.Fatalf("plugins = %d, want 2: %#v", len(got.Plugins), got.Plugins)
	}
	if got.Plugins[0].Name != "alpha" || got.Plugins[1].Name != "zeta" {
		t.Fatalf("plugins not sorted: %#v", got.Plugins)
	}
	if got.Plugins[1].Source.Path != "./dist/codex-plugins/zeta" {
		t.Fatalf("source path = %q", got.Plugins[1].Source.Path)
	}
	if got.Plugins[0].Policy.Installation != "AVAILABLE" || got.Plugins[0].Policy.Authentication != "ON_INSTALL" {
		t.Fatalf("policy mismatch: %#v", got.Plugins[0].Policy)
	}
	if got.Plugins[0].Category != defaultCodexCategory {
		t.Fatalf("empty category fallback = %q", got.Plugins[0].Category)
	}
}
