package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/installplan"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func samplePlan() *installplan.Plan {
	return &installplan.Plan{
		Runtime: model.RuntimeCodex,
		Steps: []installplan.Step{
			{Bundle: "rev", Name: "session", Type: model.TypeSkill, Files: []installplan.AdaptedFile{
				{Path: ".agents/skills/session/SKILL.md", Kind: "file", Payload: "session\n"},
			}},
			{Bundle: "rev", Name: "bq", Type: model.TypeMCP, Files: []installplan.AdaptedFile{
				{Path: "config.toml", Kind: "mergeTOMLKey", Key: "mcp_servers.bq",
					Payload: "command = \"node\"\nargs = [\"bq.js\"]\n"},
			}},
		},
	}
}

func TestInstallThenRemoveLeavesClean(t *testing.T) {
	dest := t.TempDir()
	// pre-existing user config.toml the install must preserve
	cfg := filepath.Join(dest, "config.toml")
	os.WriteFile(cfg, []byte("# mine\nlog_level=\"info\"\n"), 0o644)

	res, err := Install(dest, samplePlan(), Options{Force: false})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if got, _ := os.ReadFile(cfg); !contains(string(got), "mcp_servers.bq") || !contains(string(got), "# mine") {
		t.Fatalf("toml merge wrong:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dest, ".agents/skills/session/SKILL.md")); err != nil {
		t.Fatalf("skill not written: %v", err)
	}

	if err := Remove(dest, res.ManifestPath); err != nil {
		t.Fatalf("remove: %v", err)
	}
	// user content survives, managed table gone, managed file gone
	got, _ := os.ReadFile(cfg)
	if !contains(string(got), "# mine") || contains(string(got), "mcp_servers.bq") {
		t.Fatalf("remove did not excise precisely:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dest, ".agents/skills/session/SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("managed file should be gone after remove")
	}
}

func TestInstallIdempotent(t *testing.T) {
	dest := t.TempDir()
	if _, err := Install(dest, samplePlan(), Options{}); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dest, "config.toml")
	first, _ := os.ReadFile(cfg)
	if _, err := Install(dest, samplePlan(), Options{}); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(cfg)
	if string(first) != string(second) {
		t.Fatalf("re-install not idempotent:\n--1--\n%s\n--2--\n%s", first, second)
	}
}

func TestInstallRefusesUnmanagedCollision(t *testing.T) {
	dest := t.TempDir()
	// user already has an UNMANAGED mcp_servers.bq table
	os.WriteFile(filepath.Join(dest, "config.toml"),
		[]byte("[mcp_servers.bq]\ncommand=\"theirs\"\n"), 0o644)
	_, err := Install(dest, samplePlan(), Options{Force: false})
	if err == nil {
		t.Fatal("expected collision refusal without --force")
	}
	if ie, ok := err.(*ConflictError); !ok || ie == nil {
		t.Fatalf("want ConflictError, got %T %v", err, err)
	}
	// --force overwrites
	if _, err := Install(dest, samplePlan(), Options{Force: true}); err != nil {
		t.Fatalf("force install failed: %v", err)
	}
}

func TestRepairAfterCrashMidInstall(t *testing.T) {
	dest := t.TempDir()
	cfg := filepath.Join(dest, "config.toml")
	os.WriteFile(cfg, []byte("# mine\n"), 0o644)
	// simulate crash: journal exists uncommitted + a managed file partially written
	jp := filepath.Join(dest, ".stark", "install.journal")
	os.MkdirAll(filepath.Dir(jp), 0o755)
	j, _ := OpenJournal(jp)
	j.Record(JournalEntry{Op: "write", Path: ".agents/skills/session/SKILL.md"})
	j.Record(JournalEntry{Op: "mergeTOML", Path: "config.toml", Key: "mcp_servers.bq"})
	j.Close() // NOT committed
	os.MkdirAll(filepath.Join(dest, ".agents/skills/session"), 0o755)
	os.WriteFile(filepath.Join(dest, ".agents/skills/session/SKILL.md"), []byte("partial\n"), 0o644)

	if err := Repair(dest); err != nil {
		t.Fatalf("repair: %v", err)
	}
	// partial file rolled back, user content intact, journal cleared
	if _, err := os.Stat(filepath.Join(dest, ".agents/skills/session/SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("repair should have removed the partial file")
	}
	got, _ := os.ReadFile(cfg)
	if !contains(string(got), "# mine") {
		t.Fatalf("repair clobbered user content:\n%s", got)
	}
	if _, err := os.Stat(jp); !os.IsNotExist(err) {
		t.Fatal("journal should be cleared after repair")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
