package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/load"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/validate"
)

// Schema-valid source fields (version/tags/category/maturity/summary/runtimes) must be CARRIED,
// not silently discarded then misreported as "defaulted".
func TestMapCommonFrontmatterCarriesSchemaFields(t *testing.T) {
	a := &model.Artifact{Type: model.TypeSkill}
	raw := map[string]any{
		"name": "x", "description": "d",
		"version":  "2.5.0",
		"maturity": "stable",
		"category": "code-review",
		"summary":  "s",
		"tags":     []any{"pr", "review"},
		"runtimes": []any{"claude", "codex"},
	}
	mapCommonFrontmatter(a, raw)
	if a.Version != "2.5.0" || a.Maturity != model.MaturityStable || a.Category != "code-review" || a.Summary != "s" {
		t.Fatalf("scalar source fields not carried: %+v", a)
	}
	if len(a.Tags) != 2 || a.Tags[0] != "pr" {
		t.Fatalf("tags not carried: %+v", a.Tags)
	}
	if len(a.Runtimes) != 2 || a.Runtimes[1] != model.RuntimeCodex {
		t.Fatalf("runtimes not carried: %+v", a.Runtimes)
	}
	// and applyArtifactDefaults must NOT default/note a field the source supplied
	res := &ImportResult{}
	applyArtifactDefaults(a, res, "b/skill/x")
	for _, n := range res.Notes {
		if n.Field == "version" || n.Field == "maturity" {
			t.Fatalf("source-supplied field wrongly noted as defaulted: %+v", n)
		}
	}
}

// Every unmapped source key (e.g. context: fork) must be recorded, not silently dropped.
func TestNoteUnmappedFields(t *testing.T) {
	res := &ImportResult{}
	noteUnmappedFields(map[string]any{
		"name": "x", "context": "fork", "revision": "abc", "model": "opus",
	}, res, "b/skill/x")
	got := map[string]bool{}
	for _, n := range res.Notes {
		got[n.Field] = true
	}
	if !got["context"] || !got["revision"] {
		t.Fatalf("unmapped keys not noted: %+v", res.Notes)
	}
	if got["name"] || got["model"] {
		t.Fatalf("handled keys wrongly noted: %+v", res.Notes)
	}
}

// A name-less SKILL.md (identity from the directory) must import with the derived name, record
// a note, AND validate clean through the real load+validate pipeline.
func TestNameLessSkillDerivesNameAndValidates(t *testing.T) {
	res, err := Import(Options{From: "testdata/stark-skills", Bundle: "demo-skills"})
	if err != nil {
		t.Fatal(err)
	}
	a := findArtifact(res.Bundle, "demo-noname")
	if a == nil {
		t.Fatal("name-less skill did not get the directory-derived name 'demo-noname'")
	}
	var nameNote, contextNote bool
	for _, n := range res.Notes {
		if n.Where == "demo-skills/skill/demo-noname" && n.Field == "name" {
			nameNote = true
		}
		if n.Where == "demo-skills/skill/demo-noname" && n.Field == "context" {
			contextNote = true
		}
	}
	if !nameNote {
		t.Fatal("derived name not recorded as a note")
	}
	if !contextNote {
		t.Fatal("source-only `context` not recorded as a note")
	}
	dst := t.TempDir()
	if err := WriteBundle(res, dst); err != nil {
		t.Fatal(err)
	}
	cat, err := load.Load(dst)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if r := validate.Catalog(cat); r.HasErrors() {
		t.Fatalf("name-less-derived bundle has validation errors: %+v", r.Errors)
	}
}

// WriteBundle must refuse a path-traversal / non-slug artifact name (fail closed before writing).
func TestWriteBundleRejectsTraversalName(t *testing.T) {
	res := &ImportResult{Bundle: &model.Bundle{
		Name: "b", Version: "0.1.0", Owner: model.Owner{Name: "x"},
		Artifacts: []*model.Artifact{
			{Name: "../../evil", Type: model.TypeSkill, Description: "d", Version: "0.1.0", Body: "x\n"},
		},
	}}
	dst := t.TempDir()
	if err := WriteBundle(res, dst); err == nil {
		t.Fatal("WriteBundle must reject a traversal artifact name")
	}
	// nothing escaped the dest
	if _, err := os.Stat(filepath.Join(dst, "evil.md")); !os.IsNotExist(err) {
		t.Fatal("a file escaped the bundle dir")
	}
}

// serializeMCP must emit maturity so the on-disk file matches its defaulted IMPORT-NOTE.
func TestSerializeMCPEmitsMaturity(t *testing.T) {
	a := &model.Artifact{
		Name: "gh", Type: model.TypeMCP, Description: "GitHub MCP.", Version: "0.1.0",
		Maturity: model.MaturityBeta, Runtimes: []model.Runtime{model.RuntimeClaude},
		MCP: &model.MCPConfig{Transport: "stdio", Command: "node"},
	}
	out, err := serializeArtifact(a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "maturity: beta") {
		t.Fatalf("mcp yaml missing maturity:\n%s", out)
	}
}
