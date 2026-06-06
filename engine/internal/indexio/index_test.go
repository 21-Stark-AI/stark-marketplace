package indexio

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestLoadIndex(t *testing.T) {
	idx, err := LoadIndex("testdata/index.json")
	if err != nil {
		t.Fatal(err)
	}
	if idx.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d", idx.SchemaVersion)
	}
	if len(idx.Artifacts) != 2 {
		t.Fatalf("want 2 artifacts, got %d", len(idx.Artifacts))
	}
	for _, e := range idx.Artifacts {
		if e.Type == "bundle" {
			t.Fatalf("lean index must not contain bundle-type rows: %+v", e)
		}
	}
	mcp := idx.Find("stark-gh", "gh", model.TypeMCP)
	if mcp == nil || mcp.Support[model.RuntimeCodex] != model.SupportNative {
		t.Fatalf("mcp entry lookup failed: %+v", mcp)
	}
}

func TestLoadBundleDetail(t *testing.T) {
	d, err := LoadBundleDetail("testdata/bundles", "stark-gh")
	if err != nil {
		t.Fatal(err)
	}
	if d.Bundle.Name != "stark-gh" || d.Bundle.Version != "0.1.0" || d.Bundle.Maturity != model.MaturityBeta {
		t.Fatalf("bundle meta wrong: %+v", d.Bundle)
	}
	if len(d.Artifacts) != 2 {
		t.Fatalf("want 2 artifacts, got %d", len(d.Artifacts))
	}
	a := d.Artifact("gh", model.TypeMCP)
	if a == nil || a.MCP == nil || a.MCP.Command != "node" {
		t.Fatalf("mcp artifact detail wrong: %+v", a)
	}
	if a.Diverged {
		t.Fatalf("gh should not be diverged: %+v", a)
	}
	out := a.Outputs[model.RuntimeCodex]
	if len(out) != 1 || out[0].Kind != "mergeTOMLKey" || out[0].Key != "mcp_servers.gh" {
		t.Fatalf("codex output wrong: %+v", out)
	}
}

func TestAssertSchemaVersion(t *testing.T) {
	if err := AssertSchemaVersion(1); err != nil {
		t.Fatalf("v1 should be supported: %v", err)
	}
	if err := AssertSchemaVersion(99); err == nil {
		t.Fatal("v99 must be unsupported")
	}
}
