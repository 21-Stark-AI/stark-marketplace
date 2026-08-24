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
		Artifacts: []*model.Artifact{
			{Name: "cleanup", Type: model.TypeCommand, Runtimes: []model.Runtime{model.RuntimeCodex}},
		},
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

func TestGenerateCodexPluginManifestPointsAtMCPConfig(t *testing.T) {
	b := &model.Bundle{
		Name: "stark-brain",
		Artifacts: []*model.Artifact{
			{Name: "brain", Type: model.TypeMCP, Runtimes: []model.Runtime{model.RuntimeCodex}},
		},
	}

	got, err := GenerateCodexPlugin(b, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if got.MCPServers != "./.mcp.json" || got.Skills != "" {
		t.Fatalf("MCP-only manifest paths are wrong: %#v", got)
	}
	body, err := MarshalCodex(got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"mcpServers": "./.mcp.json"`) || strings.Contains(string(body), `"skills"`) {
		t.Fatalf("MCP-only manifest JSON is wrong:\n%s", body)
	}
}

func TestGenerateCodexPluginRejectsInvalidVersion(t *testing.T) {
	_, err := GenerateCodexPlugin(&model.Bundle{Name: "demo"}, "dev")
	if err == nil || !strings.Contains(err.Error(), "strict semver") {
		t.Fatalf("want strict semver error, got %v", err)
	}
}

func TestGenerateCodexPluginTranslatesKnownInvocationSyntax(t *testing.T) {
	b := &model.Bundle{
		Name:        "stark-implement",
		Description: "Use /stark-build or /stark-copilot; preserve /claude-only and docs/stark-build.",
		Artifacts: []*model.Artifact{
			{Name: "stark-build", Runtimes: []model.Runtime{model.RuntimeCodex}},
			{Name: "stark-copilot", Runtimes: []model.Runtime{model.RuntimeCodex}},
			{Name: "claude-only", Runtimes: []model.Runtime{model.RuntimeClaude}},
		},
	}
	want := "Use $stark-build or $stark-copilot; preserve /claude-only and docs/stark-build."

	got, err := GenerateCodexPlugin(b, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != want || got.Interface.LongDescription != want {
		t.Fatalf("Codex manifest descriptions were not translated:\n%#v", got)
	}
	if !strings.Contains(got.Interface.ShortDescription, "$stark-build") || strings.Contains(got.Interface.ShortDescription, "Use /stark-build") {
		t.Fatalf("Codex interface short description retained slash syntax: %q", got.Interface.ShortDescription)
	}
	if b.Description == want || !strings.Contains(b.Description, "/stark-build") {
		t.Fatalf("Codex projection mutated catalog metadata: %q", b.Description)
	}
}

func TestGenerateCodexPluginTranslatesQualifiedCommands(t *testing.T) {
	b := &model.Bundle{
		Name:        "stark-gh",
		Description: "Use /stark-gh:cleanup, /stark-gh:pr-open, or /stark-gh:pr-merge; not /stark-gh:review.",
		Artifacts: []*model.Artifact{
			{Name: "cleanup", Type: model.TypeCommand, Runtimes: []model.Runtime{model.RuntimeCodex}},
			{Name: "pr-open", Type: model.TypeCommand, Runtimes: []model.Runtime{model.RuntimeCodex}},
			{Name: "pr-merge", Type: model.TypeCommand, Runtimes: []model.Runtime{model.RuntimeCodex}},
			{Name: "review", Type: model.TypeSkill, Runtimes: []model.Runtime{model.RuntimeCodex}},
		},
	}
	want := "Use $cleanup, $pr-open, or $pr-merge; not /stark-gh:review."

	got, err := GenerateCodexPlugin(b, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != want || got.Interface.LongDescription != want || got.Interface.ShortDescription != want {
		t.Fatalf("qualified command syntax was not projected into every Codex manifest description:\n%#v", got)
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
