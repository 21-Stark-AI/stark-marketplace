package build

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/marketplace"
	"github.com/21StarkCom/bifrost/engine/internal/model"
)

func TestBuildProducesNativeCodexPluginAndMarketplace(t *testing.T) {
	cat := codexTestCatalog()
	out, err := Build(cat, Options{CodexPluginVersion: "1.4.2"})
	if err != nil {
		t.Fatal(err)
	}

	skillPath := "dist/codex-plugins/demo/skills/demo-skill/SKILL.md"
	if body, ok := out.Files[skillPath]; !ok {
		t.Fatalf("native Codex skill missing: %s", skillPath)
	} else if !strings.HasPrefix(string(body), "---\nname: demo-skill\n") {
		t.Fatalf("unexpected rendered skill:\n%s", body)
	}

	pluginPath := "dist/codex-plugins/demo/.codex-plugin/plugin.json"
	var plugin marketplace.CodexPluginManifest
	if err := json.Unmarshal(out.Files[pluginPath], &plugin); err != nil {
		t.Fatalf("plugin manifest invalid: %v", err)
	}
	if plugin.Name != "demo" || plugin.Version != "1.4.2" || plugin.Skills != "./skills/" {
		t.Fatalf("plugin identity mismatch: %#v", plugin)
	}
	if len(plugin.Interface.DefaultPrompt) == 0 || len(plugin.Interface.Capabilities) == 0 {
		t.Fatalf("required interface metadata missing: %#v", plugin.Interface)
	}

	var manifest marketplace.CodexMarketplaceManifest
	if err := json.Unmarshal(out.Files[marketplace.CodexManifestRelPath], &manifest); err != nil {
		t.Fatalf("marketplace manifest invalid: %v", err)
	}
	if len(manifest.Plugins) != 1 || manifest.Plugins[0].Source.Path != "./dist/codex-plugins/demo" {
		t.Fatalf("marketplace source mismatch: %#v", manifest.Plugins)
	}
	if manifest.Interface.DisplayName != "21 Stark" {
		t.Fatalf("marketplace display name = %q", manifest.Interface.DisplayName)
	}
	if _, leaked := out.Files["dist/codex-plugins/demo/.agents/skills/demo-skill/SKILL.md"]; leaked {
		t.Fatal("plugin package leaked standalone .agents/skills layout")
	}
}

func TestCodexAssetLayersDoNotAffectClaude(t *testing.T) {
	cat := codexTestCatalog()
	shared := t.TempDir()
	plugins := t.TempDir()
	codexOnly := t.TempDir()
	mustWrite(t, filepath.Join(shared, "config.json"), "shared\n")
	mustWrite(t, filepath.Join(shared, "shared.txt"), "shared-only\n")
	mustWrite(t, filepath.Join(shared, "references", "shared.md"), "Run ${CLAUDE_PLUGIN_ROOT}/tools/shared.ts.\n")
	mustWrite(t, filepath.Join(shared, "tools", "literal.ts"), "const root = '${CLAUDE_PLUGIN_ROOT}';\n")
	mustWrite(t, filepath.Join(plugins, "demo", "config.json"), "plugin\n")
	mustWrite(t, filepath.Join(plugins, "demo", "plugin.txt"), "plugin-only\n")
	mustWrite(t, filepath.Join(plugins, "demo", "references", "plugin.markdown"), "Run ${CLAUDE_PLUGIN_ROOT}/tools/plugin.ts.\n")
	mustWrite(t, filepath.Join(codexOnly, "demo", "config.json"), "codex\n")
	mustWrite(t, filepath.Join(codexOnly, "demo", "codex.txt"), "codex-only\n")
	mustWrite(t, filepath.Join(codexOnly, "demo", "references", "codex.md"), "Run ${CLAUDE_PLUGIN_ROOT}/tools/codex.ts.\n")
	// Rendered artifacts must win even if an overlay tries to occupy the path.
	mustWrite(t, filepath.Join(codexOnly, "demo", "skills", "demo-skill", "SKILL.md"), "not-rendered\n")

	base, err := Build(cat, Options{AssetsSource: shared, PluginAssetsRoot: plugins})
	if err != nil {
		t.Fatal(err)
	}
	withCodex, err := Build(cat, Options{
		AssetsSource: shared, PluginAssetsRoot: plugins, CodexAssetsRoot: codexOnly,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertFile(t, withCodex, "dist/claude/demo/config.json", "plugin\n")
	if _, ok := withCodex.Files["dist/claude/demo/codex.txt"]; ok {
		t.Fatal("Codex-only asset leaked into dist/claude")
	}
	assertFile(t, withCodex, "dist/codex-plugins/demo/config.json", "codex\n")
	assertFile(t, withCodex, "dist/codex-plugins/demo/shared.txt", "shared-only\n")
	assertFile(t, withCodex, "dist/codex-plugins/demo/plugin.txt", "plugin-only\n")
	assertFile(t, withCodex, "dist/codex-plugins/demo/codex.txt", "codex-only\n")
	assertFile(t, withCodex, "dist/claude/demo/references/shared.md", "Run ${CLAUDE_PLUGIN_ROOT}/tools/shared.ts.\n")
	for _, path := range []string{
		"dist/codex-plugins/demo/references/shared.md",
		"dist/codex-plugins/demo/references/plugin.markdown",
		"dist/codex-plugins/demo/references/codex.md",
	} {
		if body := string(withCodex.Files[path]); strings.Contains(body, "CLAUDE_PLUGIN_ROOT") {
			t.Fatalf("Claude runtime root leaked into Codex prose asset %s: %q", path, body)
		}
	}
	assertFile(t, withCodex, "dist/codex-plugins/demo/tools/literal.ts", "const root = '${CLAUDE_PLUGIN_ROOT}';\n")
	if got := string(withCodex.Files["dist/codex-plugins/demo/skills/demo-skill/SKILL.md"]); got == "not-rendered\n" {
		t.Fatal("Codex asset overlay won over the rendered skill")
	}

	// The additive Codex overlay path must leave every Claude/index-owned byte
	// exactly as it was, including the legacy marketplace manifest and bundle
	// digests used by check-bumps.
	for path, before := range base.Files {
		if !strings.HasPrefix(path, "dist/claude/") &&
			!strings.HasPrefix(path, "bundles/") &&
			path != "index.json" && path != marketplace.ManifestRelPath {
			continue
		}
		if after := withCodex.Files[path]; string(after) != string(before) {
			t.Fatalf("Codex overlay changed Claude output %s", path)
		}
	}
}

func TestBuildCodexPluginFailsClosedOnMCP(t *testing.T) {
	cat := &model.Catalog{Bundles: []*model.Bundle{{
		Name: "mcp-demo", Version: "0.1.0", Description: "MCP demo",
		Owner: model.Owner{Name: "Example"},
		Artifacts: []*model.Artifact{{
			Name: "server", Type: model.TypeMCP, Version: "0.1.0",
			Description: "Server", Runtimes: []model.Runtime{model.RuntimeCodex},
			MCP: &model.MCPConfig{Transport: "stdio", Command: "server"},
			Raw: map[string]any{"name": "server", "type": "mcp"},
		}},
	}}}
	_, err := Build(cat, Options{})
	if err == nil || !strings.Contains(err.Error(), "plugin-root .mcp.json aggregation") {
		t.Fatalf("want fail-closed MCP error, got %v", err)
	}
}

func TestCodexGeneratedRootsParticipateInWriteAndDrift(t *testing.T) {
	root := t.TempDir()
	unrelatedSkill := filepath.Join(root, ".agents", "skills", "local", "SKILL.md")
	mustWrite(t, unrelatedSkill, "local\n")
	out := Output{Files: map[string][]byte{
		"dist/codex-plugins/demo/.codex-plugin/plugin.json": []byte("{}\n"),
		marketplace.CodexManifestRelPath:                    []byte("{}\n"),
	}}
	if err := Write(root, out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unrelatedSkill); err != nil {
		t.Fatalf("build removed unrelated local .agents content: %v", err)
	}
	extra := filepath.Join(root, "dist", "codex-plugins", "demo", "stale.txt")
	mustWrite(t, extra, "stale\n")
	drift, err := Check(root, out)
	if err != nil {
		t.Fatal(err)
	}
	want := "dist/codex-plugins/demo/stale.txt (unexpected)"
	if !containsString(drift, want) {
		t.Fatalf("drift = %v, want %q", drift, want)
	}
}

func codexTestCatalog() *model.Catalog {
	return &model.Catalog{Bundles: []*model.Bundle{{
		Name: "demo", Version: "0.1.0", Description: "Demo workflows for Codex.",
		Category: "productivity", Tags: []string{"demo"},
		Owner: model.Owner{Name: "Example Team", Email: "team@example.com"},
		Artifacts: []*model.Artifact{{
			Name: "demo-skill", Type: model.TypeSkill, Version: "0.1.0",
			Description: "Run the demo workflow.",
			Runtimes:    []model.Runtime{model.RuntimeClaude, model.RuntimeCodex},
			Raw: map[string]any{
				"name": "demo-skill", "type": "skill",
				"description": "Run the demo workflow.", "version": "0.1.0",
			},
			Body: "Follow the demo workflow.\n",
		}},
	}}}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
