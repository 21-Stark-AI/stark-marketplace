package main

import (
	"os"
	"os/exec"
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

// TestCodexToolsResolveVendoredAssetRoot is the run-a-tool half of the gate: with the
// SKILL-body's STARK_ASSET_ROOT pointing at the vendored bundle root, the tool's OWN
// asset_root_lib.assetRoot() must resolve config/tools THERE — not to ~/.claude/code-review.
// Static path checks can't catch a resolution bug inside the tool; this executes it.
func TestCodexToolsResolveVendoredAssetRoot(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH — skipping tool-resolution exec check")
	}
	dest := liveCodexInstall(t, "stark-gh")
	assetRoot := filepath.Join(dest, ".agents", "stark", "stark-gh")
	lib := filepath.Join(assetRoot, "tools", "asset_root_lib.ts")
	if _, err := os.Stat(lib); err != nil {
		t.Fatalf("vendored asset_root_lib.ts missing: %v", err)
	}
	// Import the INSTALLED lib and assert its resolvers land inside the vendored root.
	// Import specifiers and the root literal are single-quoted; temp/install paths
	// never contain a single quote.
	script := `import { assetRoot, assetConfigPath, assetToolsDir } from 'file://` + filepath.ToSlash(lib) + `';
import fs from 'node:fs';
const root = '` + filepath.ToSlash(assetRoot) + `';
for (const [label, p] of [['assetRoot', assetRoot()], ['config', assetConfigPath()], ['tools', assetToolsDir()]]) {
  if (!p.startsWith(root)) { console.error(label + ' escaped vendored root: ' + p); process.exit(2); }
}
if (!fs.existsSync(assetConfigPath())) { console.error('config.json missing at ' + assetConfigPath()); process.exit(3); }
if (!fs.existsSync(assetToolsDir())) { console.error('tools/ missing at ' + assetToolsDir()); process.exit(4); }
`
	cmd := exec.Command(node, "--experimental-strip-types", "--no-warnings", "--input-type=module", "-e", script)
	// Reproduce the SKILL-body invocation env: STARK_ASSET_ROOT set to the bundle root,
	// STARK_STATE_ROOT redirected so the test never touches the real ~/.claude tree.
	cmd.Env = append(os.Environ(),
		"STARK_ASSET_ROOT="+assetRoot,
		"STARK_STATE_ROOT="+t.TempDir(),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tool asset resolution failed: %v\n%s", err, out)
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
