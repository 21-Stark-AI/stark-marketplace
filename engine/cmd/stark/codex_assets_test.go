package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/indexio"
	"github.com/21StarkCom/bifrost/engine/internal/install"
	"github.com/21StarkCom/bifrost/engine/internal/installplan"
	"github.com/21StarkCom/bifrost/engine/internal/model"
)

// liveCodexInstall installs a bundle from the COMMITTED catalog + vendor snapshot
// into a temp dest and returns the dest root.
func liveCodexInstall(t *testing.T, bundle string) string {
	t.Helper()
	root := repoRoot(t)
	idx, err := indexio.LoadIndex(filepath.Join(root, "index.json"))
	if err != nil {
		t.Skipf("committed index.json not present (%v)", err)
	}
	vendor := filepath.Join(root, "vendor", "stark-skills")
	if !dirExists(vendor) {
		t.Skipf("committed vendor snapshot not present — skipping")
	}
	ad := realAdapter(filepath.Join(root, "catalog"), vendor, filepath.Join(root, "vendor", "plugins"))
	p, err := installplan.Compute(idx, filepath.Join(root, "bundles"), ad, bundle, "",
		model.TypeCommand, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if _, err := install.Install(dest, p, install.Options{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	return dest
}

// relRefRe / varRefRe mirror the two reference shapes a skill body can carry after
// the codex target retargets them. toolSigRe is the stark-tool runner signature;
// retargetedToolRe is that signature with its mandatory inline asset and state
// roots. Keeping the two assignments adjacent to the invocation makes each
// shell call self-contained.
var (
	relRefRe         = regexp.MustCompile(`\.\./\.\./[A-Za-z0-9_./-]+`)
	varRefRe         = regexp.MustCompile(`\$\{STARK_PLUGIN_ROOT:-\$HOME/([^}]*)\}([A-Za-z0-9_./-]*)`)
	toolSigRe        = regexp.MustCompile(`node --experimental-strip-types`)
	retargetedToolRe = regexp.MustCompile(`env STARK_ASSET_ROOT="[^"]*" STARK_STATE_ROOT="\$\{STARK_STATE_ROOT:-\$HOME/\.stark/code-review\}" node --experimental-strip-types`)
)

// TestCodexInstallResolvesEveryAssetReference is the live-surface gate this whole
// change exists for: every asset path a rendered SKILL.md names must exist on disk
// after `stark install --runtime codex`. Before the fix, all 29 artifacts referenced
// files that were never written.
func TestCodexInstallResolvesEveryAssetReference(t *testing.T) {
	for _, bundle := range []string{"stark-plan", "stark-ops"} {
		t.Run(bundle, func(t *testing.T) {
			dest := liveCodexInstall(t, bundle)
			skills, err := filepath.Glob(filepath.Join(dest, ".agents/skills/*/SKILL.md"))
			if err != nil || len(skills) == 0 {
				t.Fatalf("no skills installed (%v)", err)
			}
			checked := 0
			for _, sk := range skills {
				body, err := os.ReadFile(sk)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(body), "CLAUDE_PLUGIN_ROOT") {
					t.Errorf("%s still references CLAUDE_PLUGIN_ROOT — unset on Codex", sk)
				}
				dir := filepath.Dir(sk)
				for _, ref := range relRefRe.FindAllString(string(body), -1) {
					ref = strings.TrimRight(ref, ".,)")
					if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(ref))); err != nil {
						t.Errorf("%s: dangling relative reference %q", sk, ref)
					}
					checked++
				}
				for _, m := range varRefRe.FindAllStringSubmatch(string(body), -1) {
					// m[1] is the $HOME-relative fallback root, m[2] the trailing path.
					rel := strings.TrimRight(m[1]+m[2], ".,)")
					if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel))); err != nil {
						t.Errorf("%s: dangling asset reference %q", sk, rel)
					}
					checked++
				}
				// Every stark-tool invocation must carry inline asset and state roots;
				// without them the invocation can lose its packaged assets or let mutable
				// Codex state escape the ~/.stark tree. A bare signature is the exact
				// false-green this gate previously missed.
				if bare, retargeted := len(toolSigRe.FindAllString(string(body), -1)), len(retargetedToolRe.FindAllString(string(body), -1)); bare != retargeted {
					t.Errorf("%s: %d tool invocations, only %d carry Codex asset and state roots", sk, bare, retargeted)
				}
			}
			if checked == 0 {
				t.Fatalf("no asset references found in %d skills — the check is vacuous", len(skills))
			}
		})
	}
}

// A skills-only bundle must get the SHARED plugin config.json, not a per-bundle
// override: stark-ops carries no plugin config of its own, so it inherits the
// shared snapshot (domain_agents, never a draft override).
func TestCodexPluginConfigDoesNotClobberShared(t *testing.T) {
	opsDest := liveCodexInstall(t, "stark-ops")
	opsCfg, err := os.ReadFile(filepath.Join(opsDest, ".agents/stark/stark-ops/config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(opsCfg), `"draft"`) || !strings.Contains(string(opsCfg), `"domain_agents"`) {
		t.Fatalf("stark-ops must get the SHARED config.json: %s", opsCfg)
	}
}
