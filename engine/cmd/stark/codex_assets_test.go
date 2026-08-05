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
// the codex target retargets them.
var (
	relRefRe = regexp.MustCompile(`\.\./\.\./[A-Za-z0-9_./-]+`)
	varRefRe = regexp.MustCompile(`\$\{STARK_PLUGIN_ROOT:-\$HOME/([^}]*)\}([A-Za-z0-9_./-]*)`)
)

// TestCodexInstallResolvesEveryAssetReference is the live-surface gate this whole
// change exists for: every asset path a rendered SKILL.md names must exist on disk
// after `stark install --runtime codex`. Before the fix, all 29 artifacts referenced
// files that were never written.
func TestCodexInstallResolvesEveryAssetReference(t *testing.T) {
	for _, bundle := range []string{"stark-gh", "stark-plan", "stark-ops"} {
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
			}
			if checked == 0 {
				t.Fatalf("no asset references found in %d skills — the check is vacuous", len(skills))
			}
		})
	}
}

// A bundle with its own plugin config.json must NOT clobber the shared snapshot's,
// which is why assets are bundle-scoped rather than dumped into a flat .agents/.
func TestCodexPluginConfigDoesNotClobberShared(t *testing.T) {
	dest := liveCodexInstall(t, "stark-gh")
	ghCfg, err := os.ReadFile(filepath.Join(dest, ".agents/stark/stark-gh/config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ghCfg), `"draft"`) {
		t.Fatalf("stark-gh's own config.json did not win: %s", ghCfg)
	}
	opsDest := liveCodexInstall(t, "stark-ops")
	opsCfg, err := os.ReadFile(filepath.Join(opsDest, ".agents/stark/stark-ops/config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(opsCfg), `"draft"`) || !strings.Contains(string(opsCfg), `"domain_agents"`) {
		t.Fatalf("stark-ops must get the SHARED config.json: %s", opsCfg)
	}
}

// Assets are manifest-tracked like any other managed file: re-installing must not
// trip the unmanaged-collision guard, and --remove must take them away.
func TestCodexAssetsAreManagedAndRemovable(t *testing.T) {
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
	plan := func() *installplan.Plan {
		p, err := installplan.Compute(idx, filepath.Join(root, "bundles"), ad, "stark-gh", "",
			model.TypeCommand, model.RuntimeCodex)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	dest := t.TempDir()
	if _, err := install.Install(dest, plan(), install.Options{}); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(dest, ".agents/stark/stark-gh/tools/gh_cleanup.ts")
	if _, err := os.Stat(probe); err != nil {
		t.Fatalf("plugin tool not installed: %v", err)
	}
	res, err := install.Install(dest, plan(), install.Options{})
	if err != nil {
		t.Fatalf("re-install must be idempotent, not a collision: %v", err)
	}
	if err := install.Remove(dest, res.ManifestPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(probe); err == nil {
		t.Fatal("--remove left a vendored asset behind")
	}
}
