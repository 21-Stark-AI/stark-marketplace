package codex

import (
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/model"
)

// TestAssetPathSplitsPerSkillFromBundleScoped pins the two-way mapping: per-skill
// assets must land NEXT TO their skill (Codex skills live at .agents/skills/<name>/),
// everything else under the bundle's own root so two installed bundles never fight
// over config.json.
func TestAssetPathSplitsPerSkillFromBundleScoped(t *testing.T) {
	cases := []struct{ rel, want string }{
		{"tools/gh_cleanup.ts", ".agents/stark/stark-gh/tools/gh_cleanup.ts"},
		{"standards/help.md", ".agents/stark/stark-gh/standards/help.md"},
		{"config.json", ".agents/stark/stark-gh/config.json"},
		{"skills/stark-logging/references/x.md", ".agents/skills/stark-logging/references/x.md"},
	}
	for _, c := range cases {
		if got := AssetPath("stark-gh", c.rel); got != c.want {
			t.Errorf("AssetPath(%q) = %q, want %q", c.rel, got, c.want)
		}
	}
}

// TestRetargetPluginRootBothForms covers the plain and defaulted forms of the Claude
// variable. Neither is ever set on Codex, so both must point at the bundle root that
// BundleAssets installs.
func TestRetargetPluginRootBothForms(t *testing.T) {
	in := "TOOLS=\"${CLAUDE_PLUGIN_ROOT}/tools\"\n" +
		"node ${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/code-review}/tools/jury.ts\n"
	got := retargetAssets(in, "stark-gh")
	if strings.Contains(got, "CLAUDE_PLUGIN_ROOT") {
		t.Fatalf("Claude plugin var survived: %q", got)
	}
	want := "${STARK_PLUGIN_ROOT:-$HOME/.agents/stark/stark-gh}"
	if strings.Count(got, want) != 2 {
		t.Fatalf("both forms must retarget to %q; got %q", want, got)
	}
}

// TestRetargetRelativeAssetRefs: on Claude a skill sits at <plugin>/skills/<n>/SKILL.md
// so ../../ is the plugin root; on Codex it sits at .agents/skills/<n>/SKILL.md so ../../
// is .agents/ — one level short. The rewrite must insert the bundle root.
func TestRetargetRelativeAssetRefs(t *testing.T) {
	in := "see [help](../../standards/help.md) and ../../tools/preflight.ts\n"
	got := retargetAssets(in, "stark-plan")
	for _, want := range []string{"../../stark/stark-plan/standards/help.md", "../../stark/stark-plan/tools/preflight.ts"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

// TestRetargetLeavesUnrelatedRelativePathsAlone guards against over-matching: a
// ../../ path that is not one of the vendored asset dirs is the author's own repo
// reference and must survive untouched.
func TestRetargetLeavesUnrelatedRelativePathsAlone(t *testing.T) {
	in := "../../docs/adr/0001-x.md and ../../CLAUDE.md\n"
	if got := retargetAssets(in, "stark-ops"); got != in {
		t.Fatalf("rewrote an unrelated path: %q", got)
	}
}

// TestRenderRetargetsSkillBody is the end-to-end proof through the target: a rendered
// SKILL.md carries no reference that only resolves inside a Claude plugin.
func TestRenderRetargetsSkillBody(t *testing.T) {
	a := &model.Artifact{
		Name: "stark-author", Type: model.TypeSkill, Bundle: "stark-plan",
		Description: "Author a spec.", Version: "0.1.0",
		Body:     "follow [help](../../standards/help.md)\nnode ${CLAUDE_PLUGIN_ROOT}/tools/x.ts\n",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := findFile(files, ".agents/skills/stark-author/SKILL.md")
	if !ok {
		t.Fatalf("no skill rendered: %v", files)
	}
	if strings.Contains(body, "CLAUDE_PLUGIN_ROOT") || strings.Contains(body, "../../standards/") {
		t.Fatalf("body still points at a Claude plugin root: %q", body)
	}
}
