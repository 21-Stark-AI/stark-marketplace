package marketplace

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestManifestJSONShape(t *testing.T) {
	m := Manifest{
		Name:  "stark-marketplace",
		Owner: Owner{Name: "21 Stark AI", Email: "engineering@21stark.com"},
		Plugins: []Plugin{{
			Name:        "stark-gh",
			Source:      Source{Path: "./dist/claude/stark-gh"},
			Description: "GitHub workflow commands.",
			Version:     "0.1.0",
			Author:      Owner{Name: "21 Stark AI", Email: "engineering@21stark.com"},
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
	// CC object sources are a discriminated union keyed by `source`.
	b, err := json.Marshal(Source{Type: "github", Repo: "21-Stark-AI/stark-marketplace", Ref: "main"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"source":"github"`) {
		t.Fatalf("object source must carry the source discriminator: %s", s)
	}
	if !strings.Contains(s, `"repo":"21-Stark-AI/stark-marketplace"`) {
		t.Fatalf("github source must carry repo: %s", s)
	}
	if !strings.Contains(s, `"ref":"main"`) {
		t.Fatalf("optional ref must be carried: %s", s)
	}
	// git-subdir carries url + path.
	gs, _ := json.Marshal(Source{Type: "git-subdir", URL: "https://github.com/x/y", SubPath: "dist/claude/z"})
	if !strings.Contains(string(gs), `"source":"git-subdir"`) || !strings.Contains(string(gs), `"path":"dist/claude/z"`) {
		t.Fatalf("git-subdir source shape wrong: %s", gs)
	}
}

func twoBundleCatalog() *model.Catalog {
	// Per-bundle owners are DISTINCT from each other and from the root owner so the
	// owner@root vs author@entry mapping is actually guarded (red-team Part B).
	return &model.Catalog{Bundles: []*model.Bundle{
		// intentionally out of sorted order to prove deterministic sort:
		{
			Name: "stark-gh", Version: "0.1.0", Description: "GitHub workflow.",
			Category: "productivity", Tags: []string{"github", "pr"},
			Owner: model.Owner{Name: "GH Team", Email: "gh@21stark.com"},
		},
		{
			Name: "alpha-bundle", Version: "1.2.0", Description: "Alpha tools.",
			Category: "examples", Tags: []string{"demo"},
			Owner: model.Owner{Name: "Alpha Team", Email: "alpha@21stark.com"},
		},
	}}
}

func defaultOpts() Options {
	return Options{
		Name:     "stark-marketplace",
		Owner:    Owner{Name: "21 Stark AI Platform", Email: "platform@21stark.com"},
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
	if p.Version != "0.1.0" || p.Category != "productivity" {
		t.Fatalf("entry fields wrong: %+v", p)
	}
	if !p.Strict {
		t.Fatal("strict must default to true")
	}
	// author@entry derives from the BUNDLE owner; owner@root from Options — they differ.
	if p.Author.Name != "GH Team" {
		t.Fatalf("entry author must derive from the bundle owner, got %+v", p.Author)
	}
	if m.Owner.Name != "21 Stark AI Platform" {
		t.Fatalf("root owner must be the Options owner, got %+v", m.Owner)
	}
	if p.Author.Name == m.Owner.Name {
		t.Fatal("entry author must be distinct from root owner in this fixture (guards owner/author mapping)")
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
