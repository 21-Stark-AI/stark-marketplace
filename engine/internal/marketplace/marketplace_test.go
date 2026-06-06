package marketplace

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestManifestJSONShape(t *testing.T) {
	m := Manifest{
		Name:  "stark-marketplace",
		Owner: Owner{Name: "Evinced", Email: "engineering@evinced.com"},
		Plugins: []Plugin{{
			Name:        "stark-gh",
			Source:      Source{Path: "./dist/claude/stark-gh"},
			Description: "GitHub workflow commands.",
			Version:     "0.1.0",
			Author:      Owner{Name: "Evinced", Email: "engineering@evinced.com"},
			Category:    "productivity",
			Tags:        []string{"github", "pr"},
			Strict:      true,
		}},
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"owner":`) {
		t.Fatalf("root must use owner: %s", s)
	}
	entry := s[strings.Index(s, `"plugins":`):]
	if !strings.Contains(entry, `"author":`) {
		t.Fatalf("plugin entry must use author: %s", entry)
	}
	if strings.Contains(entry, `"owner":`) {
		t.Fatalf("plugin entry must NOT use owner: %s", entry)
	}
}

func TestSourceStringForm(t *testing.T) {
	b, err := json.Marshal(Source{Path: "./dist/claude/stark-gh"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"./dist/claude/stark-gh"` {
		t.Fatalf("string source must marshal as a bare string, got %s", b)
	}
}

func TestSourceObjectForm(t *testing.T) {
	b, err := json.Marshal(Source{GitHub: "GetEvinced/stark-marketplace", GitSubdir: "dist/claude/stark-gh"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"github":"GetEvinced/stark-marketplace"`) {
		t.Fatalf("object source must carry github field: %s", s)
	}
}

func twoBundleCatalog() *model.Catalog {
	return &model.Catalog{Bundles: []*model.Bundle{
		// intentionally out of sorted order to prove deterministic sort:
		{
			Name: "stark-gh", Version: "0.1.0", Description: "GitHub workflow.",
			Category: "productivity", Tags: []string{"github", "pr"},
			Owner: model.Owner{Name: "Evinced", Email: "engineering@evinced.com"},
		},
		{
			Name: "alpha-bundle", Version: "1.2.0", Description: "Alpha tools.",
			Category: "examples", Tags: []string{"demo"},
			Owner: model.Owner{Name: "Evinced", Email: "engineering@evinced.com"},
		},
	}}
}

func defaultOpts() Options {
	return Options{
		Name:     "stark-marketplace",
		Owner:    Owner{Name: "Evinced", Email: "engineering@evinced.com"},
		DistRoot: "./dist/claude",
	}
}

func TestGenerateOneEntryPerBundleSorted(t *testing.T) {
	m := Generate(twoBundleCatalog(), defaultOpts())
	if len(m.Plugins) != 2 {
		t.Fatalf("want 2 plugins, got %d", len(m.Plugins))
	}
	if m.Plugins[0].Name != "alpha-bundle" || m.Plugins[1].Name != "stark-gh" {
		t.Fatalf("plugins not sorted by name: %+v", m.Plugins)
	}
	p := m.Plugins[1]
	if p.Source.Path != "./dist/claude/stark-gh" {
		t.Fatalf("source path = %q", p.Source.Path)
	}
	if p.Author.Name != "Evinced" || p.Version != "0.1.0" || p.Category != "productivity" {
		t.Fatalf("entry fields wrong: %+v", p)
	}
	if !p.Strict {
		t.Fatal("strict must default to true")
	}
	if m.Owner.Name != "Evinced" {
		t.Fatalf("root owner = %+v", m.Owner)
	}
}

func TestGoldenMarshal(t *testing.T) {
	m := Generate(twoBundleCatalog(), defaultOpts())
	got, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	goldenPath := filepath.Join("testdata", "marketplace.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		_ = os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSchemaShapeContract(t *testing.T) {
	m := Generate(twoBundleCatalog(), defaultOpts())
	raw, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"name", "owner", "plugins"} {
		if _, ok := doc[k]; !ok {
			t.Fatalf("root missing required field %q", k)
		}
	}
	owner, ok := doc["owner"].(map[string]any)
	if !ok || owner["name"] == nil {
		t.Fatalf("root owner must be an object with name: %v", doc["owner"])
	}
	if _, hasAuthor := doc["author"]; hasAuthor {
		t.Fatal("root must NOT carry author (owner only)")
	}
	plugins, ok := doc["plugins"].([]any)
	if !ok || len(plugins) == 0 {
		t.Fatal("plugins must be a non-empty array")
	}
	for i, pany := range plugins {
		p := pany.(map[string]any)
		for _, k := range []string{"name", "source", "version", "author"} {
			if _, ok := p[k]; !ok {
				t.Fatalf("plugin %d missing required field %q", i, k)
			}
		}
		auth, ok := p["author"].(map[string]any)
		if !ok || auth["name"] == nil {
			t.Fatalf("plugin %d author must be an object with name", i)
		}
		if _, hasOwner := p["owner"]; hasOwner {
			t.Fatalf("plugin %d must NOT carry owner (author only)", i)
		}
		switch src := p["source"].(type) {
		case string:
			if src == "" {
				t.Fatalf("plugin %d empty string source", i)
			}
		case map[string]any:
			if src["github"] == nil && src["url"] == nil && src["git-subdir"] == nil {
				t.Fatalf("plugin %d object source missing github/url/git-subdir", i)
			}
		default:
			t.Fatalf("plugin %d source has wrong type %T", i, p["source"])
		}
	}
}
