package main

import (
	"path/filepath"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/indexio"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/install"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/installplan"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// TestE2EPlanInstallDoctorRemove drives the same code paths the cobra commands call,
// asserting the exit-code contract (§9.8) end to end against a temp dest.
func TestE2EPlanInstallDoctorRemove(t *testing.T) {
	idx, err := indexio.LoadIndex("../../internal/install/testdata/index.json")
	if err != nil {
		t.Fatal(err)
	}
	fa := installplan.NewFakeAdapter(map[string]string{
		"config.toml#mcp_servers.srv": "command = \"node\"\n",
	})
	p, err := installplan.Compute(idx, "../../internal/install/testdata/bundles", fa,
		"multi", "srv", model.TypeMCP, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Consent.Required {
		t.Fatal("mcp install must require consent")
	}
	dest := t.TempDir()
	res, err := install.Install(dest, p, install.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rep, _ := install.Doctor(dest, res.ManifestPath)
	if len(rep.Broken) != 0 {
		t.Fatalf("doctor broken: %+v", rep.Broken)
	}
	// second install is idempotent (no conflict, since we own it)
	if _, err := install.Install(dest, p, install.Options{}); err != nil {
		t.Fatalf("idempotent re-install failed: %v", err)
	}
	if err := install.Remove(dest, res.ManifestPath); err != nil {
		t.Fatal(err)
	}
	_ = filepath.Join // anchor
}

func TestE2ESchemaVersionExit(t *testing.T) {
	if checkIndexSupported(indexio.SchemaVersionMax+1) != ExitSchemaVersion {
		t.Fatal("out-of-range schemaVersion must map to exit 5")
	}
}
