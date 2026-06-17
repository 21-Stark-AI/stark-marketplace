package build

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/load"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestBuildTwiceIdentical(t *testing.T) {
	cat, err := load.Load("../../../catalog")
	if err != nil {
		t.Fatal(err)
	}
	a, err := Build(cat, Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(cat, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Files) != len(b.Files) {
		t.Fatalf("file count differs: %d vs %d", len(a.Files), len(b.Files))
	}
	for p, av := range a.Files {
		if string(b.Files[p]) != string(av) {
			t.Fatalf("file %s differs between builds", p)
		}
	}
}

func TestSourceKeyReorderIdentical(t *testing.T) {
	// Same artifact, frontmatter Raw map keys inserted in different order.
	mk := func(raw map[string]any) *model.Catalog {
		return &model.Catalog{Bundles: []*model.Bundle{{
			Name: "demo", Version: "0.1.0", Description: "d", Owner: model.Owner{Name: "E"},
			Runtimes: []model.Runtime{model.RuntimeClaude},
			Artifacts: []*model.Artifact{{
				Name: "rev", Type: model.TypeCommand, Description: "c", Version: "0.1.0",
				ArgumentHint: "[PR]", Model: "opus",
				Runtimes: []model.Runtime{model.RuntimeClaude}, Raw: raw, Body: "body\n",
			}},
		}}}
	}
	c1 := mk(map[string]any{"name": "rev", "model": "opus", "argument-hint": "[PR]"})
	c2 := mk(map[string]any{"argument-hint": "[PR]", "model": "opus", "name": "rev"})
	o1, _ := Build(c1, Options{})
	o2, _ := Build(c2, Options{})
	for p, v := range o1.Files {
		if string(o2.Files[p]) != string(v) {
			t.Fatalf("key reorder changed %s:\n%s\nvs\n%s", p, v, o2.Files[p])
		}
	}
}

// TestWriteNormalizesToLF proves no generated file on disk contains a CR, even if
// a source body smuggled in CRLF (F-Cov#7 / spec §7.6 LF determinism).
func TestWriteNormalizesToLF(t *testing.T) {
	root := t.TempDir()
	out := Output{Files: map[string][]byte{
		"dist/claude/x/skills/y/SKILL.md": []byte("---\nname: y\r\n---\r\nbody\r\n"),
		"index.json":                      []byte("{\r\n  \"schemaVersion\": 1\r\n}\r\n"),
	}}
	if err := Write(root, out); err != nil {
		t.Fatal(err)
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if bytes.ContainsRune(b, '\r') {
			t.Fatalf("generated file %s contains a CR", path)
		}
		return nil
	})
}
