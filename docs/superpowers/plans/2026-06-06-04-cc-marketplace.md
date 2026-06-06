# stark-marketplace — Slice 4: CC marketplace generator + native install loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Generate `dist/claude/.claude-plugin/marketplace.json` in the **native** Claude Code marketplace format — one `plugins[]` entry per bundle, root `owner`, entry-level `author`, correct `source` — wire it into `stark build` so it is produced and drift-checked alongside `dist/claude/` and `index.json`, and document + verify the end-to-end native install loop (`/plugin marketplace add` → `/plugin install <bundle>`).

**Architecture:** A pure Go package `engine/internal/marketplace` projects the loaded `model.Catalog` (slice 1) and the lean index (slice 2) into the CC marketplace manifest. Generation is a deterministic projection: one entry per bundle, `source` pointing at the bundle's committed `dist/claude/<bundle>/` tree (relative path within the repo, so `/plugin marketplace add GetEvinced/stark-marketplace` resolves locally). `stark build` calls the generator after emitting the Claude dist tree + index, writes the manifest into the committed tree, and `stark build --check` fails on drift. A schema-shape test asserts the manifest matches the real CC marketplace contract (root `owner`, entry `author`, required `source`/`version`).

**Tech Stack:** Go 1.23 (pinned `toolchain`), standard `encoding/json` (ordered struct encoding → deterministic), `gopkg.in/yaml.v3` (already a dep), standard `testing`.

**Depends on:** Slice 1 (`model.*`, `load.Load`) — consumed verbatim. Slice 2 (Claude adapter → `dist/claude/<bundle>/...`, lean `index.json`, `index.Index`/`index.BundleEntry`, and the `stark build` command). **This slice projects directly from `model.Bundle` and does NOT read the index — it does not depend on the index's internal shape (e.g. the CC-2 top-level `artifacts` key); if a future change makes it consume the index, it must use the canonical `artifacts` key, not `entries`.** Task 2's projector reads only `bundle.Name`/`Version`/`Description`/`Category`/`Tags`/`Owner` from `model.Bundle` directly, so this slice lands even if slice 2's index types are not yet present.

---

## A. File / package structure

```
engine/
  internal/
    marketplace/
      marketplace.go            # manifest types + Generate() (Task 1,2)
      marketplace_test.go       # unit + golden + schema-shape (Task 1,2,3,4)
      write.go                  # WriteManifest + path constants (Task 5)
      testdata/
        marketplace.golden.json # byte-exact golden (Task 3)
  cmd/stark/
    build.go                    # MODIFY: call marketplace gen + drift-check (Task 6)
    build_marketplace_test.go   # build emits + drift-checks manifest (Task 6)
docs/
  native-install-loop.md        # documented + scripted install loop (Task 7)
  scripts/
    verify-native-install.sh    # smoke script for the install loop (Task 7)
```

The native CC marketplace contract this slice targets (corrected per spec §8 / red-team Part B):

```jsonc
{
  "name": "stark-marketplace",
  "owner": { "name": "Evinced", "email": "engineering@evinced.com" },   // ROOT uses owner
  "plugins": [
    {
      "name": "stark-gh",
      "source": "./dist/claude/stark-gh",   // string form OR object {github|url|git-subdir}
      "description": "GitHub workflow commands + MCP for stark.",
      "version": "0.1.0",
      "author": { "name": "Evinced", "email": "engineering@evinced.com" }, // ENTRY uses author
      "category": "productivity",
      "tags": ["github", "pr", "workflow"],
      "strict": true
    }
  ]
}
```

Every step below runs from the repo root unless noted. Go commands run from `engine/`.

---

### Task 1: Manifest types (root `owner`, entry `author`)

**Files:**
- Create: `engine/internal/marketplace/marketplace.go`
- Test: `engine/internal/marketplace/marketplace_test.go`

- [ ] **Step 1: Write the failing test pinning owner-vs-author at the type level**

`engine/internal/marketplace/marketplace_test.go`:
```go
package marketplace

import (
	"encoding/json"
	"strings"
	"testing"
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
	// ROOT key must be "owner", never "author".
	if !strings.Contains(s, `"owner":`) {
		t.Fatalf("root must use owner: %s", s)
	}
	// The plugin ENTRY must use "author", never "owner".
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
	if !strings.Contains(s, `"source":"GetEvinced/stark-marketplace"`) &&
		!strings.Contains(s, `"github":"GetEvinced/stark-marketplace"`) {
		t.Fatalf("object source must carry github field: %s", s)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/marketplace/ -run 'TestManifest|TestSource' -v`
Expected: FAIL — undefined `Manifest`/`Owner`/`Plugin`/`Source`.

- [ ] **Step 3: Implement the manifest types with a polymorphic Source**

`engine/internal/marketplace/marketplace.go`:
```go
// Package marketplace projects the catalog into the native Claude Code
// marketplace manifest (dist/claude/.claude-plugin/marketplace.json).
//
// CRITICAL CONTRACT (spec §8, red-team Part B):
//   - The manifest ROOT uses `owner` (name/email).
//   - Each plugins[] ENTRY uses `author` (NOT owner), plus source/version/
//     description/category/tags/strict.
// These two keys are deliberately distinct types/fields so the distinction
// cannot rot.
package marketplace

import "encoding/json"

// Owner identifies a maintainer (root `owner`) or plugin author (entry `author`).
type Owner struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// Source locates a plugin's committed tree. It marshals as a bare string when
// only Path is set (string form), otherwise as the object form
// {github|url|git-subdir}. Exactly one form is emitted per source.
type Source struct {
	Path      string `json:"-"` // string form: relative repo path, e.g. ./dist/claude/stark-gh
	GitHub    string `json:"github,omitempty"`
	URL       string `json:"url,omitempty"`
	GitSubdir string `json:"git-subdir,omitempty"`
}

// MarshalJSON emits the string form when Path is set, else the object form.
func (s Source) MarshalJSON() ([]byte, error) {
	if s.Path != "" {
		return json.Marshal(s.Path)
	}
	type obj struct {
		GitHub    string `json:"github,omitempty"`
		URL       string `json:"url,omitempty"`
		GitSubdir string `json:"git-subdir,omitempty"`
	}
	return json.Marshal(obj{GitHub: s.GitHub, URL: s.URL, GitSubdir: s.GitSubdir})
}

// Plugin is one plugins[] entry — exactly one bundle. Uses `author`, not `owner`.
type Plugin struct {
	Name        string   `json:"name"`
	Source      Source   `json:"source"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version"`
	Author      Owner    `json:"author"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Strict      bool     `json:"strict"`
}

// Manifest is the whole .claude-plugin/marketplace.json. Root uses `owner`.
type Manifest struct {
	Name    string   `json:"name"`
	Owner   Owner    `json:"owner"`
	Plugins []Plugin `json:"plugins"`
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/marketplace/ -run 'TestManifest|TestSource' -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add engine/internal/marketplace/marketplace.go engine/internal/marketplace/marketplace_test.go
git commit -m "feat(marketplace): native CC manifest types (root owner, entry author)"
```

---

### Task 2: Generate() — project the catalog into a deterministic manifest

**Files:**
- Modify: `engine/internal/marketplace/marketplace.go`
- Test: `engine/internal/marketplace/marketplace_test.go`

- [ ] **Step 1: Write the failing generation test**

Append to `engine/internal/marketplace/marketplace_test.go`:
```go
import (
	// add to the existing import block:
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

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

func TestGenerateOneEntryPerBundleSorted(t *testing.T) {
	m := Generate(twoBundleCatalog(), Options{
		Name:     "stark-marketplace",
		Owner:    Owner{Name: "Evinced", Email: "engineering@evinced.com"},
		DistRoot: "./dist/claude",
	})
	if len(m.Plugins) != 2 {
		t.Fatalf("want 2 plugins, got %d", len(m.Plugins))
	}
	// deterministic: sorted by bundle name regardless of catalog order
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/marketplace/ -run TestGenerate -v`
Expected: FAIL — undefined `Generate`/`Options`.

- [ ] **Step 3: Implement Generate + Options**

Append to `engine/internal/marketplace/marketplace.go`:
```go
import (
	// extend the existing import block:
	"path"
	"sort"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// Options configures manifest generation. Pure inputs only — no clock/env.
type Options struct {
	Name     string // marketplace name, e.g. "stark-marketplace"
	Owner    Owner  // ROOT owner
	DistRoot string // relative path to the committed claude dist, e.g. "./dist/claude"
}

// Generate projects a loaded catalog into the native CC manifest. One plugins[]
// entry per bundle, sorted by bundle name for determinism (spec §7.6). The
// per-bundle source points at the committed dist/claude/<bundle>/ tree.
func Generate(cat *model.Catalog, opts Options) Manifest {
	bundles := append([]*model.Bundle(nil), cat.Bundles...)
	sort.Slice(bundles, func(i, j int) bool { return bundles[i].Name < bundles[j].Name })

	m := Manifest{Name: opts.Name, Owner: opts.Owner}
	for _, b := range bundles {
		m.Plugins = append(m.Plugins, Plugin{
			Name:        b.Name,
			Source:      Source{Path: path.Join(opts.DistRoot, b.Name)},
			Description: b.Description,
			Version:     b.Version,
			Author:      Owner{Name: b.Owner.Name, Email: b.Owner.Email},
			Category:    b.Category,
			Tags:        append([]string(nil), b.Tags...),
			Strict:      true,
		})
	}
	return m
}
```

> **Note on `path.Join`:** it strips the leading `./`. The test expects
> `./dist/claude/stark-gh`. Keep the `./` prefix by constructing the source as
> `opts.DistRoot + "/" + b.Name` when `DistRoot` starts with `./`. Use this
> exact body instead of `path.Join` to preserve the prefix and `/` separators
> (spec §7.6):
> ```go
> Source: Source{Path: opts.DistRoot + "/" + b.Name},
> ```
> Drop the `"path"` import if it becomes unused.

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/marketplace/ -run TestGenerate -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/marketplace/
git commit -m "feat(marketplace): Generate() — deterministic one-entry-per-bundle projection"
```

---

### Task 3: Golden test for the serialized manifest

**Files:**
- Create: `engine/internal/marketplace/testdata/marketplace.golden.json`
- Test: `engine/internal/marketplace/marketplace_test.go`

- [ ] **Step 1: Write the failing golden test**

Append to `engine/internal/marketplace/marketplace_test.go`:
```go
import (
	// extend the existing import block:
	"bytes"
	"os"
	"path/filepath"
)

// Marshal returns the canonical (indented, LF, trailing-newline) JSON bytes.
// This is the exact serializer Generate's callers must use so the golden and
// the committed dist file stay byte-identical (spec §7.6).
func TestGoldenMarshal(t *testing.T) {
	m := Generate(twoBundleCatalog(), Options{
		Name:     "stark-marketplace",
		Owner:    Owner{Name: "Evinced", Email: "engineering@evinced.com"},
		DistRoot: "./dist/claude",
	})
	got, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	goldenPath := filepath.Join("testdata", "marketplace.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
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
```

- [ ] **Step 2: Implement Marshal (canonical encoder)**

Append to `engine/internal/marketplace/marketplace.go`:
```go
import (
	// extend the existing import block:
	"bytes"
)

// Marshal serializes a manifest deterministically: 2-space indent, no HTML
// escaping, LF line endings, single trailing newline. This is THE canonical
// encoder for both the golden test and the committed dist file.
func Marshal(m Manifest) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil { // Encode appends a trailing newline
		return nil, err
	}
	return buf.Bytes(), nil
}
```

- [ ] **Step 3: Generate the golden, then run the test to verify pass**

Run:
```bash
cd engine && UPDATE_GOLDEN=1 go test ./internal/marketplace/ -run TestGoldenMarshal && go test ./internal/marketplace/ -run TestGoldenMarshal -v && cd ..
```
Expected: first run writes the golden, second run PASSES against it.

- [ ] **Step 4: Verify the golden content is correct (manual inspection)**

Run: `cat engine/internal/marketplace/testdata/marketplace.golden.json`
Expected (byte-exact — root `owner`, entries sorted `alpha-bundle` then `stark-gh`, each entry `author` not `owner`, string `source`):
```json
{
  "name": "stark-marketplace",
  "owner": {
    "name": "Evinced",
    "email": "engineering@evinced.com"
  },
  "plugins": [
    {
      "name": "alpha-bundle",
      "source": "./dist/claude/alpha-bundle",
      "description": "Alpha tools.",
      "version": "1.2.0",
      "author": {
        "name": "Evinced",
        "email": "engineering@evinced.com"
      },
      "category": "examples",
      "tags": [
        "demo"
      ],
      "strict": true
    },
    {
      "name": "stark-gh",
      "source": "./dist/claude/stark-gh",
      "description": "GitHub workflow.",
      "version": "0.1.0",
      "author": {
        "name": "Evinced",
        "email": "engineering@evinced.com"
      },
      "category": "productivity",
      "tags": [
        "github",
        "pr"
      ],
      "strict": true
    }
  ]
}
```

- [ ] **Step 5: Commit**

```bash
git add engine/internal/marketplace/
git commit -m "test(marketplace): byte-exact golden for serialized manifest"
```

---

### Task 4: Schema-shape test against the real CC marketplace contract

**Files:**
- Test: `engine/internal/marketplace/marketplace_test.go`

This is the verification gate from the slice scope: assert the generated manifest validates against the real CC marketplace schema **shape** (required fields present, `author` at entry level, `owner` at root). We assert the contract structurally over the decoded JSON rather than depending on a remote schema file (offline, deterministic).

- [ ] **Step 1: Write the failing schema-shape test**

Append to `engine/internal/marketplace/marketplace_test.go`:
```go
func TestSchemaShapeContract(t *testing.T) {
	m := Generate(twoBundleCatalog(), Options{
		Name:     "stark-marketplace",
		Owner:    Owner{Name: "Evinced", Email: "engineering@evinced.com"},
		DistRoot: "./dist/claude",
	})
	raw, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}

	// ROOT required fields per CC marketplace schema.
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
		// ENTRY required fields.
		for _, k := range []string{"name", "source", "version", "author"} {
			if _, ok := p[k]; !ok {
				t.Fatalf("plugin %d missing required field %q", i, k)
			}
		}
		// author at entry level, NEVER owner.
		auth, ok := p["author"].(map[string]any)
		if !ok || auth["name"] == nil {
			t.Fatalf("plugin %d author must be an object with name", i)
		}
		if _, hasOwner := p["owner"]; hasOwner {
			t.Fatalf("plugin %d must NOT carry owner (author only)", i)
		}
		// source is a string OR an object with github/url/git-subdir.
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
```

- [ ] **Step 2: Run to verify it passes**

Run: `cd engine && go test ./internal/marketplace/ -run TestSchemaShape -v`
Expected: PASS (the types from Tasks 1–2 already satisfy the contract; this test locks it).

- [ ] **Step 3: Commit**

```bash
git add engine/internal/marketplace/marketplace_test.go
git commit -m "test(marketplace): schema-shape contract (owner@root, author@entry, source required)"
```

---

### Task 5: WriteManifest — emit into the committed Claude dist tree

**Files:**
- Create: `engine/internal/marketplace/write.go`
- Test: `engine/internal/marketplace/write_test.go`

- [ ] **Step 1: Write the failing write/round-trip test**

`engine/internal/marketplace/write_test.go`:
```go
package marketplace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteManifestRoundTrips(t *testing.T) {
	dir := t.TempDir()
	m := Generate(twoBundleCatalog(), Options{
		Name:     "stark-marketplace",
		Owner:    Owner{Name: "Evinced", Email: "engineering@evinced.com"},
		DistRoot: "./dist/claude",
	})
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ManifestRelPath))
	if err != nil {
		t.Fatalf("manifest not at expected path: %v", err)
	}
	want, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("written bytes differ from canonical Marshal")
	}
}

func TestReadManifestForDrift(t *testing.T) {
	dir := t.TempDir()
	m := Generate(twoBundleCatalog(), Options{Name: "x", Owner: Owner{Name: "E"}, DistRoot: "./d"})
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	b, err := ReadManifestBytes(dir)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := Marshal(m)
	if string(b) != string(want) {
		t.Fatal("ReadManifestBytes must return the on-disk bytes verbatim")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/marketplace/ -run 'TestWriteManifest|TestReadManifest' -v`
Expected: FAIL — undefined `WriteManifest`/`ManifestRelPath`/`ReadManifestBytes`.

- [ ] **Step 3: Implement write.go**

`engine/internal/marketplace/write.go`:
```go
package marketplace

import (
	"os"
	"path/filepath"
)

// ManifestRelPath is the manifest's location relative to the Claude dist root
// (spec §5). CC reads dist/claude/.claude-plugin/marketplace.json from the repo.
const ManifestRelPath = ".claude-plugin/marketplace.json"

// WriteManifest serializes m with the canonical encoder and writes it under
// distClaudeDir/.claude-plugin/marketplace.json, creating parent dirs.
// Atomic temp+rename so a crash never leaves a half-written manifest.
func WriteManifest(distClaudeDir string, m Manifest) error {
	data, err := Marshal(m)
	if err != nil {
		return err
	}
	dst := filepath.Join(distClaudeDir, filepath.FromSlash(ManifestRelPath))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// ReadManifestBytes returns the on-disk manifest bytes verbatim (for drift
// comparison). Missing file is returned as the error.
func ReadManifestBytes(distClaudeDir string) ([]byte, error) {
	dst := filepath.Join(distClaudeDir, filepath.FromSlash(ManifestRelPath))
	return os.ReadFile(dst)
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/marketplace/ -v`
Expected: PASS (all marketplace tests).

- [ ] **Step 5: Commit**

```bash
git add engine/internal/marketplace/write.go engine/internal/marketplace/write_test.go
git commit -m "feat(marketplace): atomic WriteManifest + ReadManifestBytes for drift"
```

---

### Task 6: Wire the generator into `stark build` (+ `--check` drift)

**Files:**
- Modify: `engine/cmd/stark/build.go` (from slice 2)
- Test: `engine/cmd/stark/build_marketplace_test.go`

> **Slice-2 integration contract:** `stark build` (slice 2) already loads the
> catalog, emits `dist/claude/<bundle>/...`, writes `index.json`, and supports
> `--check` (drift) / `--fix`. This task inserts the marketplace step into that
> flow. Slice 2's canonical signature is `func runBuild(catalogDir, repoRoot string, check bool) int`
> in package `main`; it loads the catalog into a local `cat *model.Catalog` near
> the top. The hooks below reuse that **already-loaded `cat`** — they do NOT
> re-load the catalog. The marketplace logic itself is self-contained in the
> helpers added here.

- [ ] **Step 1: Write the failing build-integration test**

`engine/cmd/stark/build_marketplace_test.go`:
```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/load"
	"github.com/GetEvinced/stark-marketplace/engine/internal/marketplace"
)

// buildMarketplace is the helper this task adds; it must produce the manifest
// for the loaded catalog under <repoRoot>/dist/claude.
func TestBuildMarketplaceEmitsManifest(t *testing.T) {
	root := findRepoRoot(t) // defined in validate_test.go (slice 1)
	tmp := t.TempDir()

	cat, err := load.Load(filepath.Join(root, "catalog"))
	if err != nil {
		t.Fatal(err)
	}
	if err := buildMarketplace(tmp, cat); err != nil {
		t.Fatal(err)
	}
	b, err := marketplace.ReadManifestBytes(filepath.Join(tmp, "dist", "claude"))
	if err != nil {
		t.Fatalf("marketplace not emitted: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("empty manifest")
	}

	// Drift check: a re-run against the same catalog must report no drift.
	drift, err := checkMarketplaceDrift(tmp, cat)
	if err != nil {
		t.Fatal(err)
	}
	if drift {
		t.Fatal("freshly written manifest reported as drifted")
	}

	// Mutate the on-disk manifest → drift must be detected.
	dst := filepath.Join(tmp, "dist", "claude", marketplace.ManifestRelPath)
	if err := os.WriteFile(dst, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err = checkMarketplaceDrift(tmp, cat)
	if err != nil {
		t.Fatal(err)
	}
	if !drift {
		t.Fatal("expected drift after manual edit")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./cmd/stark/ -run TestBuildMarketplace -v`
Expected: FAIL — undefined `buildMarketplace`/`checkMarketplaceDrift`.

- [ ] **Step 3: Implement the build helpers**

Append to `engine/cmd/stark/build.go`:
```go
import (
	// extend the existing import block:
	"bytes"

	"github.com/GetEvinced/stark-marketplace/engine/internal/marketplace"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// marketplaceOptions are the fixed root attributes for this repo's manifest.
func marketplaceOptions() marketplace.Options {
	return marketplace.Options{
		Name:     "stark-marketplace",
		Owner:    marketplace.Owner{Name: "Evinced", Email: "engineering@evinced.com"},
		DistRoot: "./dist/claude",
	}
}

// buildMarketplace writes <repoRoot>/dist/claude/.claude-plugin/marketplace.json
// from the loaded catalog. Called after the Claude dist tree + index are emitted.
func buildMarketplace(repoRoot string, cat *model.Catalog) error {
	m := marketplace.Generate(cat, marketplaceOptions())
	return marketplace.WriteManifest(filepath.Join(repoRoot, "dist", "claude"), m)
}

// checkMarketplaceDrift reports whether the committed manifest differs from a
// fresh generation (spec §5.1: drift is a required, non-bypassable gate).
func checkMarketplaceDrift(repoRoot string, cat *model.Catalog) (bool, error) {
	want, err := marketplace.Marshal(marketplace.Generate(cat, marketplaceOptions()))
	if err != nil {
		return false, err
	}
	got, err := marketplace.ReadManifestBytes(filepath.Join(repoRoot, "dist", "claude"))
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil // missing == drift
		}
		return false, err
	}
	return !bytes.Equal(got, want), nil
}
```

- [ ] **Step 4: Call the helpers from `runBuild`**

In `runBuild(catalogDir, repoRoot string, check bool) int` (slice 2's canonical
signature), after the Claude dist tree + `index.json` are emitted, reuse the
**already-loaded `cat`** (the `*model.Catalog` slice 2 loads from `catalogDir`
near the top of `runBuild`) — do NOT re-load it:
- **Normal build (`--fix`/default):** call `buildMarketplace(repoRoot, cat)`; on error return exit `1`.
- **`--check` (drift gate):** call `checkMarketplaceDrift(repoRoot, cat)`; if it reports drift, print `drift: dist/claude/.claude-plugin/marketplace.json out of date (run stark build --fix)` and return exit `2` (the spec §9.8 drift code), consistent with the dist/index drift handling already in `runBuild`.

Concretely, insert into `runBuild` alongside the existing dist/index steps (using
the in-scope `check` parameter and `cat`):
```go
	if check {
		drift, err := checkMarketplaceDrift(repoRoot, cat)
		if err != nil {
			fmt.Println("marketplace check error:", err)
			return 1
		}
		if drift {
			fmt.Println("drift: dist/claude/.claude-plugin/marketplace.json out of date (run stark build --fix)")
			return 2
		}
	} else {
		if err := buildMarketplace(repoRoot, cat); err != nil {
			fmt.Println("marketplace write error:", err)
			return 1
		}
	}
```

- [ ] **Step 5: Run to verify pass + build the binary**

Run: `cd engine && go test ./... && go build ./... && cd ..`
Expected: PASS across all packages; binary builds.

- [ ] **Step 6: Live build — generate the committed manifest**

Run from repo root:
```bash
cd engine && go run ./cmd/stark build --fix && cd ..
cat dist/claude/.claude-plugin/marketplace.json
```
Expected: a `stark-gh` entry (the slice-1 seed bundle) with `source` `./dist/claude/stark-gh`, entry-level `author`, root `owner`. Then verify the drift gate is green:
```bash
cd engine && go run ./cmd/stark build --check && echo "drift-clean exit=$?" && cd ..
```
Expected: no drift output, `drift-clean exit=0`.

- [ ] **Step 7: Commit (including the now-committed generated manifest)**

```bash
git add engine/cmd/stark/build.go engine/cmd/stark/build_marketplace_test.go dist/claude/.claude-plugin/marketplace.json
git commit -m "feat(stark): generate + drift-check CC marketplace.json in stark build"
```

---

### Task 7: Document + script the native install loop (verification)

**Files:**
- Create: `docs/native-install-loop.md`
- Create: `docs/scripts/verify-native-install.sh`

- [ ] **Step 1: Write the install-loop doc**

`docs/native-install-loop.md`:
```markdown
# Native Claude Code install loop

`stark-marketplace` installs into Claude Code with **no custom client**. The
committed `dist/claude/` tree IS the marketplace; CC reads
`dist/claude/.claude-plugin/marketplace.json` directly from the repo.

## What is committed (spec §5.1)

- **Committed:** `dist/claude/` (incl. `.claude-plugin/marketplace.json`),
  `index.json`, `bundles/*.json` — marked `linguist-generated`.
- **NOT committed:** `dist/codex/`, `dist/gemini/` — built on `stark install`
  (no in-repo consumer).

## End-to-end loop

1. Add the marketplace (private repo; you must have Evinced repo access):
   ```
   /plugin marketplace add GetEvinced/stark-marketplace
   ```
   CC resolves `dist/claude/.claude-plugin/marketplace.json` and lists every
   bundle as an installable plugin.

2. Install a bundle (one plugins[] entry == one bundle):
   ```
   /plugin install stark-gh
   ```
   CC fetches the plugin from the entry's `source`
   (`./dist/claude/stark-gh`) and installs its skills/commands/agents/mcp.

3. Update after a marketplace change:
   ```
   /plugin marketplace update GetEvinced/stark-marketplace
   /plugin install stark-gh
   ```

## Manifest contract (why installs resolve)

- Root carries `owner` (name/email).
- Each `plugins[]` entry carries `author` (NOT owner), `source`, `version`,
  `description`, `category`, `tags`, `strict`.
- `source` points at the bundle's committed `dist/claude/<bundle>/` tree
  (string form) — or an object form `{github|url|git-subdir}` when published
  from another repo.

The manifest is generated, never hand-edited: `stark build --fix` regenerates
it; `stark build --check` fails CI on drift (exit 2).

## Local verification

Run `docs/scripts/verify-native-install.sh` from the repo root. It rebuilds the
manifest, asserts the committed copy is drift-free, and structurally validates
the install contract (owner@root, author@entry, resolvable per-bundle source
trees) without needing a live CC session.
```

- [ ] **Step 2: Write the verification script**

`docs/scripts/verify-native-install.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail

# Verifies the native CC install loop offline:
#  1. marketplace.json is drift-free vs a fresh build
#  2. root uses owner, entries use author
#  3. every entry's string source resolves to a committed dist/claude/<bundle> tree
#
# Run from the repo root.

repo_root="$(git rev-parse --show-toplevel)"
manifest="$repo_root/dist/claude/.claude-plugin/marketplace.json"

echo "==> drift check"
( cd "$repo_root/engine" && go run ./cmd/stark build --check )
echo "    drift-clean"

echo "==> manifest contract"
test -f "$manifest" || { echo "missing $manifest"; exit 1; }

# Root must carry owner, never author.
jq -e '.owner.name' "$manifest" >/dev/null || { echo "root missing owner.name"; exit 1; }
if jq -e 'has("author")' "$manifest" | grep -q true; then
  echo "root must NOT carry author"; exit 1
fi

# Every entry: author present, owner absent, source resolves.
count="$(jq '.plugins | length' "$manifest")"
echo "    $count plugin entr(y/ies)"
for i in $(seq 0 $((count - 1))); do
  name="$(jq -r ".plugins[$i].name" "$manifest")"
  jq -e ".plugins[$i].author.name" "$manifest" >/dev/null \
    || { echo "entry $name missing author.name"; exit 1; }
  jq -e ".plugins[$i].version" "$manifest" >/dev/null \
    || { echo "entry $name missing version"; exit 1; }
  if jq -e ".plugins[$i] | has(\"owner\")" "$manifest" | grep -q true; then
    echo "entry $name must NOT carry owner"; exit 1
  fi
  src="$(jq -r ".plugins[$i].source" "$manifest")"
  # string source -> relative tree must exist under the repo
  if [[ "$src" == ./* ]]; then
    tree="$repo_root/${src#./}"
    test -d "$tree" || { echo "entry $name source tree missing: $tree"; exit 1; }
    echo "    $name -> $src (resolved)"
  else
    echo "    $name -> object source (skipping local resolve)"
  fi
done

echo "==> OK: native install contract verified"
```

- [ ] **Step 3: Make it executable and run it live**

Run from repo root:
```bash
chmod +x docs/scripts/verify-native-install.sh
./docs/scripts/verify-native-install.sh
```
Expected: `==> drift check` → `drift-clean`; `==> manifest contract` lists the
`stark-gh` entry resolving to `./dist/claude/stark-gh`; ends with
`==> OK: native install contract verified`, exit 0.

> If `dist/claude/stark-gh` does not exist yet (slice 2's Claude adapter not run
> in this checkout), run `cd engine && go run ./cmd/stark build --fix && cd ..`
> first so the bundle's Claude tree is emitted, then re-run the script.

- [ ] **Step 4: Commit**

```bash
git add docs/native-install-loop.md docs/scripts/verify-native-install.sh
git commit -m "docs(marketplace): native install loop + offline verification script"
```

---

### Task 8: Full-suite green + determinism re-check

**Files:**
- (no new files — verification + commit of regenerated manifest if needed)

- [ ] **Step 1: Run the full Go suite**

Run: `cd engine && go test ./... -v && cd ..`
Expected: PASS across `model`, `fence`, `load`, `validate`, `marketplace`, `cmd/stark`.

- [ ] **Step 2: Determinism — build twice, assert identical manifest**

Run from repo root:
```bash
cd engine && go run ./cmd/stark build --fix && cd ..
sha1=$(git hash-object dist/claude/.claude-plugin/marketplace.json)
cd engine && go run ./cmd/stark build --fix && cd ..
sha2=$(git hash-object dist/claude/.claude-plugin/marketplace.json)
[ "$sha1" = "$sha2" ] && echo "deterministic: $sha1" || { echo "NONDETERMINISTIC"; exit 1; }
```
Expected: `deterministic: <sha>` — two builds produce byte-identical output.

- [ ] **Step 3: Confirm the drift gate is required-status clean**

Run: `cd engine && go run ./cmd/stark build --check && echo "exit=$?" && cd ..`
Expected: no drift, `exit=0`.

- [ ] **Step 4: Commit any regenerated manifest**

```bash
git add dist/claude/.claude-plugin/marketplace.json
git commit -m "chore(marketplace): regenerate committed manifest (deterministic)" || echo "nothing to commit"
```

---

## Self-Review (completed during authoring)

- **Spec coverage (slice 4 scope = spec §16 step 4 + §8):**
  - `internal/marketplace` generates `dist/claude/.claude-plugin/marketplace.json` in native CC format (Tasks 1,2,5) ✓
  - **Red-team correctness:** root `owner` vs entry `author` enforced at the type level (Task 1) AND asserted structurally (Task 4) ✓; `source` string OR object form (Task 1, `Source.MarshalJSON`) ✓; per-entry `version`/`description`/`category`/`tags`/`strict` (Task 1) ✓; one entry per bundle, source → committed `dist/claude/<bundle>/` (Task 2) ✓
  - Wired into `stark build` with drift check alongside dist/claude + index (Task 6) ✓
  - End-to-end native install loop documented + scripted (`/plugin marketplace add` → `/plugin install`) + Go schema-shape test (Tasks 4,7) ✓
  - `dist/claude` committed / `dist/codex|gemini` NOT committed restated (Task 7 doc, consistent with slice-1 `.gitattributes`) ✓
  - Golden test for `marketplace.json` (Task 3) ✓
- **Type consistency:** uses `model.Catalog`/`model.Bundle`/`model.Owner` and `load.Load` verbatim from slice 1; reuses `findRepoRoot` from slice-1 `validate_test.go`. No dependency on deep slice-2 index internals — `Generate` projects directly from `model.Bundle`, so this slice lands even if slice 2's `index.Index`/`index.BundleEntry` are not yet present.
- **Determinism (spec §7.6):** bundles sorted by name; single canonical `Marshal` (LF, `/` separators, no HTML escape, trailing newline) shared by golden, write, and drift; build-twice identity check (Task 8) ✓
- **Slice-2 coupling (CC-6):** Task 6 uses slice 2's canonical `runBuild(catalogDir, repoRoot string, check bool) int` and reuses its already-loaded `cat *model.Catalog` (no re-load); marketplace logic is self-contained in helpers. Marketplace output stays inside the committed `dist/claude/.claude-plugin/marketplace.json`, covered by the existing `dist/claude` drift gate (no separate output path). No dependency on the index's CC-2 `artifacts` shape — projection is straight from `model.Bundle`.
- **Placeholders:** none — every step has runnable code/commands and expected output.

---

## Execution handoff

This is slice 4 of 8 (see spec §16); it depends on slice 1 (`model`, `load`, `validate`) and slice 2 (Claude adapter dist tree + `stark build`). Recommended execution: **subagent-driven-development**, one subagent per task, review between tasks; run Task 8 last as the full-suite + determinism gate. Next: slice 5 (CLI install/doctor) consumes the same committed `dist/claude/` and manifest.
