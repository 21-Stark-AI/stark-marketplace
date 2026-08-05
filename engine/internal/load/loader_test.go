package load

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/merge"
	"github.com/21StarkCom/bifrost/engine/internal/model"
)

func TestLoadCatalog(t *testing.T) {
	cat, err := Load("testdata/catalog")
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Bundles) != 1 {
		t.Fatalf("want 1 bundle, got %d", len(cat.Bundles))
	}
	b := cat.Bundles[0]
	if b.Name != "demo" || len(b.Artifacts) != 1 {
		t.Fatalf("bundle = %+v", b)
	}
	a := b.Artifacts[0]
	if a.Name != "hello" || a.Type != model.TypeCommand {
		t.Fatalf("artifact = %+v", a)
	}
	// inheritance: category/tags/runtimes come from the bundle
	if a.Category != "examples" || len(a.Runtimes) != 3 {
		t.Fatalf("inheritance failed: %+v", a)
	}
	if a.Body != "Hello, world.\n" {
		t.Fatalf("body = %q", a.Body)
	}
}

func TestLoadSkipsNonArtifactFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "catalog", "demo", "commands")
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(dir, 0o755))
	must(os.WriteFile(filepath.Join(root, "catalog", "demo", "bundle.yaml"),
		[]byte("name: demo\nversion: 0.1.0\ndescription: d\nowner: { name: E }\nruntimes: [claude]\n"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "hello.md"),
		[]byte("---\nname: hello\ntype: command\ndescription: d\nversion: 0.1.0\n---\nbody\n"), 0o644))
	// stray files that must NOT become phantom artifacts
	must(os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("\x00\x01"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "README.txt"), []byte("notes"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "config.json"), []byte("{\"k\":1}"), 0o644))

	cat, err := Load(filepath.Join(root, "catalog"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Bundles) != 1 || len(cat.Bundles[0].Artifacts) != 1 {
		t.Fatalf("stray files not skipped: got %d artifact(s)", len(cat.Bundles[0].Artifacts))
	}
}

func TestLoadRuntimeOverrideRoundTrip(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "catalog", "demo", "skills")
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(dir, 0o755))
	must(os.WriteFile(filepath.Join(root, "catalog", "demo", "bundle.yaml"),
		[]byte("name: demo\nversion: 0.1.0\ndescription: d\nowner: { name: E }\nruntimes: [claude, codex]\n"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "review.md"), []byte(`---
name: review
type: skill
description: Claude description.
version: 0.1.0
overrides:
  codex:
    argument-hint: "[PR]"
    description: Codex description.
    body: |
      # diverged: source-owned Codex runtime override
      Codex body.
---
Claude body.
`), 0o644))

	cat, err := Load(filepath.Join(root, "catalog"))
	if err != nil {
		t.Fatal(err)
	}
	a := cat.Bundles[0].Artifacts[0]
	ov, ok := a.Overrides[model.RuntimeCodex]
	if !ok || ov.Fields["argument-hint"] != "[PR]" {
		t.Fatalf("loaded override = %+v", ov)
	}
	resolved, findings, err := merge.Resolve(a, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Frontmatter["description"] != "Codex description." || resolved.Body != "Codex body.\n" {
		t.Fatalf("resolved = %+v body=%q", resolved.Frontmatter, resolved.Body)
	}
	if !findings.Diverged || findings.DivergedReason != "source-owned Codex runtime override" {
		t.Fatalf("findings = %+v", findings)
	}
}
