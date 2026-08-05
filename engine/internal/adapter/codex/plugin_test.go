package codex

import (
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/model"
)

func TestPluginLayoutEmitsNativePluginSkill(t *testing.T) {
	a := &model.Artifact{
		Name: "cleanup", Type: model.TypeCommand, Bundle: "stark-gh",
		Description:  "Clean merged repository state safely.",
		ArgumentHint: "[--dry-run]", Body: "Run ${CLAUDE_PLUGIN_ROOT}/tools/gh_cleanup.ts.\n",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	files, _, err := NewPlugin().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := findFile(files, "skills/cleanup/SKILL.md")
	if !ok {
		t.Fatalf("native plugin skill missing: %v", files)
	}
	if strings.Contains(body, ".agents/skills") || strings.Contains(body, "CLAUDE_PLUGIN_ROOT") {
		t.Fatalf("standalone/Claude paths leaked into plugin skill:\n%s", body)
	}
	for _, want := range []string{
		"## Codex plugin asset root",
		"${STARK_PLUGIN_ROOT:?resolve from this loaded SKILL.md as instructed above}/tools/gh_cleanup.ts",
		"Usage: cleanup [--dry-run]",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("plugin skill missing %q:\n%s", want, body)
		}
	}
	policy, ok := findFile(files, "skills/cleanup/agents/openai.yaml")
	if !ok || !strings.Contains(policy, "allow_implicit_invocation: false") {
		t.Fatalf("command policy missing or implicit: %q", policy)
	}
}

func TestPluginLayoutKeepsRelativeAssetsAtPluginDepth(t *testing.T) {
	a := &model.Artifact{
		Name: "stark-author", Type: model.TypeSkill, Bundle: "stark-plan",
		Description: "Author an implementation-ready specification.",
		Body:        "Follow [help](../../standards/help.md) and [local](references/dossier.md).\n",
		Runtimes:    []model.Runtime{model.RuntimeCodex},
	}
	files, _, err := NewPlugin().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := findFile(files, "skills/stark-author/SKILL.md")
	if !ok {
		t.Fatalf("native plugin skill missing: %v", files)
	}
	if !strings.Contains(body, "../../standards/help.md") || !strings.Contains(body, "references/dossier.md") {
		t.Fatalf("plugin-relative assets were incorrectly retargeted:\n%s", body)
	}
	if strings.Contains(body, ".agents/stark/") {
		t.Fatalf("standalone asset root leaked into plugin body:\n%s", body)
	}
}

func TestPluginRetargetHandlesNestedPortableRoots(t *testing.T) {
	in := strings.Join([]string{
		`A="${STARK_ASSET_ROOT:-${STARK_PLUGIN_ROOT:-${CLAUDE_PLUGIN_ROOT:-}}}"`,
		`B="${STARK_PLUGIN_ROOT:-${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/code-review}}"`,
		`C="${STARK_PLUGIN_ROOT:-$HOME/.agents/stark/stark-gh}"`,
		`node --experimental-strip-types "$B/tools/x.ts"`,
	}, "\n")
	got := retargetPluginSkill(in)
	if strings.Contains(got, "CLAUDE_PLUGIN_ROOT") || strings.Contains(got, "$HOME/.agents/stark") {
		t.Fatalf("legacy roots survived:\n%s", got)
	}
	if strings.Contains(got, "${STARK_PLUGIN_ROOT:-${STARK_PLUGIN_ROOT") {
		t.Fatalf("nested root rewrite was malformed:\n%s", got)
	}
	if !strings.Contains(got, `STARK_ASSET_ROOT="${STARK_PLUGIN_ROOT:?resolve from this loaded SKILL.md as instructed above}" node --experimental-strip-types`) {
		t.Fatalf("tool invocation did not receive the plugin asset root:\n%s", got)
	}
}

func TestPluginLayoutRejectsStandaloneMCPConfig(t *testing.T) {
	a := &model.Artifact{
		Name: "demo", Type: model.TypeMCP, Bundle: "demo",
		Runtimes: []model.Runtime{model.RuntimeCodex},
		MCP:      &model.MCPConfig{Transport: "stdio", Command: "demo"},
	}
	_, _, err := NewPlugin().Render(bundleWith(a))
	if err == nil || !strings.Contains(err.Error(), ".mcp.json") {
		t.Fatalf("plugin MCP must fail closed, got %v", err)
	}
}

func TestRetargetPluginAssetRefsDoesNotAddSkillPreamble(t *testing.T) {
	in := "node --experimental-strip-types ${CLAUDE_PLUGIN_ROOT}/tools/x.ts and ../../tools/y.ts\n"
	got := RetargetPluginAssetRefs(in)
	if strings.Contains(got, "## Codex plugin asset root") {
		t.Fatalf("prose asset received a skill-only preamble:\n%s", got)
	}
	if strings.Contains(got, "CLAUDE_PLUGIN_ROOT") || !strings.Contains(got, "../../tools/y.ts") {
		t.Fatalf("depth-independent rewrite is wrong:\n%s", got)
	}
}
