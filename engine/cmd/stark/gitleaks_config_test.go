package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGitleaksConfigContract asserts the two contract behaviors of the repo's .gitleaks.toml
// (spec §7.4): a sanctioned `{ secretRef: <key> }` reference produces NO finding, while an inline
// literal credential DOES. The secret gate is a CLI step (no Go code path), so this shells to the
// gitleaks binary and is skipped when it is absent locally — CI installs a pinned gitleaks, so the
// assertion runs there. This is also the regression guard for the named-capture/secretGroup fix:
// without it the allowlists target the keyword instead of the value.
func TestGitleaksConfigContract(t *testing.T) {
	if _, err := exec.LookPath("gitleaks"); err != nil {
		t.Skip("gitleaks not on PATH")
	}
	cfg := filepath.Join(repoRoot(t), ".gitleaks.toml")

	scan := func(t *testing.T, name, content string) int {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("gitleaks", "dir", dir, "--config", cfg, "--redact", "--exit-code", "1")
		_ = cmd.Run() // non-zero exit on findings is expected; read the code below
		return cmd.ProcessState.ExitCode()
	}

	if code := scan(t, "ref.yaml", "env:\n  GITHUB_TOKEN: { secretRef: stark-gh-token }\n"); code != 0 {
		t.Fatalf("sanctioned secretRef must NOT trip gitleaks, got exit %d", code)
	}
	if code := scan(t, "leak.yaml", "api_key = \"ghp_live0123456789ABCDEF\"\n"); code == 0 {
		t.Fatal("inline literal credential must trip gitleaks (non-zero exit), got 0")
	}
}
