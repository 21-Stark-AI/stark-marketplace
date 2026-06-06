package install

import (
	"errors"
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

// §9.2 collision refusal must hold for the JSON kind (Claude/Gemini), not just TOML.
func TestInstallRefusesUnmanagedJSONCollision(t *testing.T) {
	dest := t.TempDir()
	os.WriteFile(filepath.Join(dest, ".mcp.json"),
		[]byte(`{"mcpServers":{"gh":{"command":"theirs"}}}`), 0o644)
	p := &installplan.Plan{Runtime: model.RuntimeClaude, Steps: []installplan.Step{
		{Bundle: "b", Name: "gh", Type: model.TypeMCP, Files: []installplan.AdaptedFile{
			{Path: ".mcp.json", Kind: "mergeJSONKey", Key: "mcpServers.gh", Payload: `{"command":"node"}`}}}}}
	if _, err := Install(dest, p, Options{}); !errors.As(err, new(*ConflictError)) {
		t.Fatalf("want ConflictError, got %T %v", err, err)
	}
	if _, err := Install(dest, p, Options{Force: true}); err != nil {
		t.Fatalf("force install failed: %v", err)
	}
}

// §9.2 collision refusal for the sentinel kind (Gemini/Claude shared MD).
func TestInstallRefusesUnmanagedSentinelCollision(t *testing.T) {
	dest := t.TempDir()
	os.WriteFile(filepath.Join(dest, "GEMINI.md"),
		[]byte("<!-- stark:begin b/agent -->\nhand-written\n<!-- stark:end b/agent -->\n"), 0o644)
	p := &installplan.Plan{Runtime: model.RuntimeGemini, Steps: []installplan.Step{
		{Bundle: "b", Name: "agent", Type: model.TypeAgent, Files: []installplan.AdaptedFile{
			{Path: "GEMINI.md", Kind: "sentinel", Sentinel: "b/agent", Payload: "role\n"}}}}}
	if _, err := Install(dest, p, Options{}); !errors.As(err, new(*ConflictError)) {
		t.Fatalf("want ConflictError, got %T %v", err, err)
	}
}

// Repair must LEAVE a committed install intact (only drop the journal) — it must act on the
// committed marker, not roll back a healthy install.
func TestRepairLeavesCommittedInstallIntact(t *testing.T) {
	dest := t.TempDir()
	jp := filepath.Join(dest, ".stark", "install.journal")
	os.MkdirAll(filepath.Dir(jp), 0o755)
	j, _ := OpenJournal(jp)
	j.Record(JournalEntry{Op: "write", Path: ".agents/skills/x/SKILL.md"})
	j.Commit() // COMMITTED
	j.Close()
	os.MkdirAll(filepath.Join(dest, ".agents/skills/x"), 0o755)
	os.WriteFile(filepath.Join(dest, ".agents/skills/x/SKILL.md"), []byte("kept\n"), 0o644)

	if err := Repair(dest); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, ".agents/skills/x/SKILL.md")); string(b) != "kept\n" {
		t.Fatalf("committed install must NOT be rolled back, got %q", b)
	}
	if _, err := os.Stat(jp); !os.IsNotExist(err) {
		t.Fatal("journal should be removed after repairing a committed run")
	}
}

// Re-running install after a crash must auto-repair the uncommitted journal (rolling back the
// prior partial) instead of truncating it and orphaning the partial mutations forever.
func TestReinstallRecoversCrashedJournal(t *testing.T) {
	dest := t.TempDir()
	os.WriteFile(filepath.Join(dest, "config.toml"), []byte("# mine\n"), 0o644)
	jp := filepath.Join(dest, ".stark", "install.journal")
	os.MkdirAll(filepath.Dir(jp), 0o755)
	j, _ := OpenJournal(jp)
	j.Record(JournalEntry{Op: "write", Path: ".agents/skills/pr-open/SKILL.md"})
	j.Close() // NOT committed → simulates a crash
	os.MkdirAll(filepath.Join(dest, ".agents/skills/pr-open"), 0o755)
	os.WriteFile(filepath.Join(dest, ".agents/skills/pr-open/SKILL.md"), []byte("partial\n"), 0o644)

	res, err := Install(dest, samplePlan(), Options{})
	if err != nil {
		t.Fatalf("re-install: %v", err)
	}
	// the crashed partial was rolled back by the auto-repair
	if _, err := os.Stat(filepath.Join(dest, ".agents/skills/pr-open/SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("auto-repair should have rolled back the crashed partial file")
	}
	// and the fresh plan applied cleanly
	if b, _ := os.ReadFile(filepath.Join(dest, ".agents/skills/session/SKILL.md")); string(b) != "session\n" {
		t.Fatalf("clean install did not complete, got %q", b)
	}
	if rep, _ := Doctor(dest, res.ManifestPath); len(rep.Broken) != 0 {
		t.Fatalf("doctor broken after recovered re-install: %+v", rep.Broken)
	}
}

// A torn write (on-disk bytes != what we wrote) surfaces as a *DigestError → exit 3 (§9.4/§9.8).
func TestWriteVerifiedDetectsTornWrite(t *testing.T) {
	orig := readBack
	defer func() { readBack = orig }()
	readBack = func(string) ([]byte, error) { return []byte("CORRUPT"), nil }
	err := writeVerified(filepath.Join(t.TempDir(), "f"), []byte("intended"))
	if !errors.As(err, new(*DigestError)) {
		t.Fatalf("want *DigestError on torn write, got %T %v", err, err)
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
