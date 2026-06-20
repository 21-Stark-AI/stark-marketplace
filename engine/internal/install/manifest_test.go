package install

import (
	"path/filepath"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{Runtime: model.RuntimeCodex, SchemaVersion: 1}
	m.Add(Record{Bundle: "stark-gh", Name: "gh", Type: model.TypeMCP,
		Path: "config.toml", Action: ActionMergeTOMLKey, Key: "mcp_servers.gh", Digest: "sha256:abc"})
	m.Add(Record{Bundle: "stark-gh", Name: "pr-open", Type: model.TypeCommand,
		Path: ".agents/skills/pr-open/SKILL.md", Action: ActionWriteFile, Digest: "sha256:def"})
	path := filepath.Join(dir, "manifest.json")
	if err := SaveManifest(path, m); err != nil {
		t.Fatal(err)
	}
	got, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 2 {
		t.Fatalf("manifest round-trip wrong: %+v", got.Records)
	}
	// sortRecords yields ascending-by-path deterministic order (idiomatic across the engine):
	// ".agents/..." < "config.toml", so the writeFile record sorts first.
	if got.Records[0].Path != ".agents/skills/pr-open/SKILL.md" || got.Records[1].Key != "mcp_servers.gh" {
		t.Fatalf("records not in deterministic ascending-path order: %+v", got.Records)
	}
	if got.SchemaVersion != 1 || got.Runtime != model.RuntimeCodex {
		t.Fatalf("manifest header not preserved: %+v", got)
	}
}
