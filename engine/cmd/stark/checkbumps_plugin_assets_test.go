package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/digest"
	"github.com/21StarkCom/bifrost/engine/internal/load"
)

// The gap this closes, observed live on 2026-07-27: a stark-skills plugin fix re-vendored
// into `vendor/plugins/stark-gh/tools/lib/git.ts` changed no ARTIFACT digest, so
// `check-bumps` reported "OK: no un-bumped source changes" and the bundle shipped modified
// content under an unchanged 0.1.10. Consumers already on 0.1.10 would never re-fetch it,
// so the fix was invisible on every installed machine.
//
// These tests pin the digest half of the gate (the plumbing half — index rows in, prev map
// out — is exercised by the repo-level `check-bumps` run in CI against the real catalog).

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestPluginAssetDigestChangesWhenAToolChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tools", "lib", "git.ts"), "export const a = 1;\n")
	writeFile(t, filepath.Join(dir, "config.json"), "{}\n")

	before, err := digest.Dir(dir)
	if err != nil {
		t.Fatalf("digest.Dir: %v", err)
	}

	// The exact shape that slipped through: one line of one vendored tool.
	writeFile(t, filepath.Join(dir, "tools", "lib", "git.ts"), "export const a = 2;\n")
	after, err := digest.Dir(dir)
	if err != nil {
		t.Fatalf("digest.Dir: %v", err)
	}
	if before == after {
		t.Fatalf("a vendored tool changed but the digest did not: %s", before)
	}
}

func TestPluginAssetDigestIsStableAcrossReads(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tools", "a.ts"), "a\n")
	writeFile(t, filepath.Join(dir, "tools", "b.ts"), "b\n")
	first, err := digest.Dir(dir)
	if err != nil {
		t.Fatalf("digest.Dir: %v", err)
	}
	second, err := digest.Dir(dir)
	if err != nil {
		t.Fatalf("digest.Dir: %v", err)
	}
	if first != second {
		t.Fatalf("digest not deterministic: %s vs %s", first, second)
	}
}

func TestPluginAssetDigestNormalizesLineEndings(t *testing.T) {
	// dist content is LF-normalized before it is written, so a CRLF checkout must not
	// read as a content change and force a spurious bump.
	lf := t.TempDir()
	crlf := t.TempDir()
	writeFile(t, filepath.Join(lf, "tools", "a.ts"), "one\ntwo\n")
	writeFile(t, filepath.Join(crlf, "tools", "a.ts"), "one\r\ntwo\r\n")
	a, err := digest.Dir(lf)
	if err != nil {
		t.Fatalf("digest.Dir: %v", err)
	}
	b, err := digest.Dir(crlf)
	if err != nil {
		t.Fatalf("digest.Dir: %v", err)
	}
	if a != b {
		t.Fatalf("line endings changed the digest: %s vs %s", a, b)
	}
}

func TestPluginAssetDigestCatchesAPureRename(t *testing.T) {
	// Paths are hashed with content, so moving a tool is a change even though the bytes
	// shipped are identical — a renamed entrypoint breaks importers just as hard.
	one := t.TempDir()
	two := t.TempDir()
	writeFile(t, filepath.Join(one, "tools", "git.ts"), "same\n")
	writeFile(t, filepath.Join(two, "tools", "git2.ts"), "same\n")
	a, _ := digest.Dir(one)
	b, _ := digest.Dir(two)
	if a == b {
		t.Fatalf("a rename produced the same digest: %s", a)
	}
}

func TestMissingPluginDirDigestsToTheEmptySet(t *testing.T) {
	// Most bundles have no vendor/plugins/<bundle> at all. That must be a stable value,
	// not an error, or the gate would fail the whole catalog.
	d, err := digest.Dir(filepath.Join(t.TempDir(), "definitely-absent"))
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if d != emptyDirDigest() {
		t.Fatalf("missing dir digest %s != empty-set digest %s", d, emptyDirDigest())
	}
}

func TestPluginAssetKeyCannotCollideWithAnArtifact(t *testing.T) {
	// Artifact keys are "<bundle>/<type>/<name>"; a collision would silently disable the
	// gate on one of the two rows (the CC-5 bypass the artifact key comment warns about).
	key := pluginAssetKey("stark-gh")
	if key != "stark-gh/plugin-assets/stark-gh" {
		t.Fatalf("unexpected key shape: %s", key)
	}
	for _, artifactType := range []string{"skill", "command", "agent", "mcp", "prompt"} {
		if key == "stark-gh/"+artifactType+"/stark-gh" {
			t.Fatalf("plugin-asset key collides with artifact type %q", artifactType)
		}
	}
}

// seedPluginAssetRepo builds a git repo whose committed index.json records a plugin-asset
// digest for bundle "demo", with `vendor/plugins/demo/tools/git.ts` on disk. `prevDigest`
// is what the committed index claims; pass a stale value to simulate "the vendored tool
// changed since the last publish".
func seedPluginAssetRepo(t *testing.T, bundleVersion, prevVersion, prevDigest string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "catalog", "demo", "bundle.yaml"),
		"name: demo\nversion: "+bundleVersion+"\ndescription: d\nowner: { name: E }\nruntimes: [claude]\n")
	writeFile(t, filepath.Join(root, "catalog", "demo", "commands", "hello.md"),
		"---\nname: hello\ntype: command\ndescription: d\nversion: 0.1.0\n---\nbody\n")
	writeFile(t, filepath.Join(root, "vendor", "plugins", "demo", "tools", "git.ts"),
		"export const a = 1;\n")

	idx := map[string]any{
		"schemaVersion": 1,
		"artifacts": []map[string]any{{
			"name": "hello", "type": "command", "bundle": "demo",
			"version": "0.1.0", "digest": digestOfHello(t, root),
		}},
		"pluginAssets": []map[string]any{{
			"bundle": "demo", "version": prevVersion, "digest": prevDigest,
		}},
	}
	b, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	writeFile(t, filepath.Join(root, "index.json"), string(b)+"\n")

	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, gitErr := cmd.CombinedOutput(); gitErr != nil {
			t.Fatalf("git %v: %v\n%s", args, gitErr, out)
		}
	}
	git("init")
	git("add", ".")
	git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "seed")
	return root
}

// digestOfHello recomputes the command artifact's source digest so the artifact half of the
// gate stays clean and only the plugin-asset half can trip.
func digestOfHello(t *testing.T, root string) string {
	t.Helper()
	cat, err := load.Load(filepath.Join(root, "catalog"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, b := range cat.Bundles {
		for _, a := range b.Artifacts {
			if a.Name == "hello" {
				return digest.Source(a)
			}
		}
	}
	t.Fatal("hello artifact not found")
	return ""
}

// THE REGRESSION TEST. A vendored plugin tool changed since the committed index, and the
// bundle version did NOT move. Before this gate existed, check-bumps printed
// "OK: no un-bumped source changes" here and the bundle shipped changed content under an
// unchanged version.
func TestCheckBumpsFailsWhenPluginToolChangedWithoutABump(t *testing.T) {
	root := seedPluginAssetRepo(t, "0.1.10", "0.1.10", "sha256:stale-from-the-last-publish")
	if code := runCheckBumps(filepath.Join(root, "catalog"), root); code != 1 {
		t.Fatalf("want exit 1 when a vendored plugin tool changed un-bumped, got %d", code)
	}
}

// Control: the SAME changed tool, with the bundle version bumped, must pass — otherwise the
// gate would be unsatisfiable and publish.sh could never go green.
func TestCheckBumpsPassesWhenPluginToolChangeIsBumped(t *testing.T) {
	root := seedPluginAssetRepo(t, "0.1.11", "0.1.10", "sha256:stale-from-the-last-publish")
	if code := runCheckBumps(filepath.Join(root, "catalog"), root); code != 0 {
		t.Fatalf("want exit 0 when the plugin-asset change carries a bump, got %d", code)
	}
}

// Control: nothing changed. The committed digest matches the tree, so an unchanged version
// is correct and must not be flagged.
func TestCheckBumpsPassesWhenPluginAssetsAreUnchanged(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "vendor", "plugins", "demo", "tools", "git.ts"),
		"export const a = 1;\n")
	current, err := digest.Dir(filepath.Join(root, "vendor", "plugins", "demo"))
	if err != nil {
		t.Fatalf("digest.Dir: %v", err)
	}
	root = seedPluginAssetRepo(t, "0.1.10", "0.1.10", current)
	if code := runCheckBumps(filepath.Join(root, "catalog"), root); code != 0 {
		t.Fatalf("want exit 0 when plugin assets are unchanged, got %d", code)
	}
}

// A previous index generated before `pluginAssets` existed must not break the gate: it
// simply contributes no previous rows, so the first publish after rollout records them.
func TestLeanPrevToleratesAnIndexWithoutPluginAssets(t *testing.T) {
	var lp leanPrev
	old := `{"schemaVersion":1,"artifacts":[{"name":"x","type":"skill","bundle":"b","version":"0.1.0","digest":"sha256:aa"}]}`
	if err := json.Unmarshal([]byte(old), &lp); err != nil {
		t.Fatalf("unmarshal legacy index: %v", err)
	}
	if len(lp.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(lp.Artifacts))
	}
	if len(lp.PluginAssets) != 0 {
		t.Fatalf("expected no plugin assets, got %d", len(lp.PluginAssets))
	}
}
