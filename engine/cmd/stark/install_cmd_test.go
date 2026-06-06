package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/indexio"
	"github.com/GetEvinced/stark-marketplace/engine/internal/install"
)

// installExitCode is the §9.8 mapping the install RunE uses; assert each typed error maps right.
func TestInstallExitCodeMapping(t *testing.T) {
	if got := installExitCode(&install.ConflictError{Msg: "x"}); got != ExitConflict {
		t.Fatalf("conflict → %d, want %d", got, ExitConflict)
	}
	if got := installExitCode(&install.DigestError{Got: "a", Want: "b"}); got != ExitDigest {
		t.Fatalf("digest → %d, want %d", got, ExitDigest)
	}
	if got := installExitCode(errors.New("some validation problem")); got != ExitValidation {
		t.Fatalf("other → %d, want %d", got, ExitValidation)
	}
}

func TestConfirmReadsAnswer(t *testing.T) {
	cases := map[string]bool{"y\n": true, "yes\n": true, "Y\n": true, "n\n": false, "\n": false, "": false}
	for in, want := range cases {
		if got := confirm(strings.NewReader(in)); got != want {
			t.Fatalf("confirm(%q) = %v, want %v", in, got, want)
		}
	}
}

// indexLoadExit must map a real schemaVersion load failure to exit 5, and a generic parse
// failure to exit 1 — this is the path search/info/install actually take (distinct from
// version.go's checkIndexSupported(int)).
func TestIndexLoadExitDiscriminates(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "index.json")
	os.WriteFile(bad, []byte(`{"schemaVersion":99,"artifacts":[]}`), 0o644)
	_, err := indexio.LoadIndex(bad)
	if err == nil {
		t.Fatal("expected schemaVersion error")
	}
	if got := indexLoadExit(err); got != ExitSchemaVersion {
		t.Fatalf("schemaVersion load → %d, want %d", got, ExitSchemaVersion)
	}

	os.WriteFile(bad, []byte(`{not json`), 0o644)
	_, err = indexio.LoadIndex(bad)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if got := indexLoadExit(err); got != ExitValidation {
		t.Fatalf("parse error → %d, want %d", got, ExitValidation)
	}
}

// Command-level: consent-required + declined drives RunE to exit 6 and writes nothing (§9.8).
func TestInstallCmdConsentDeclinedExit6(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "index.json")); err != nil {
		t.Skipf("committed index.json absent (%v)", err)
	}
	var code int
	orig := osExit
	osExit = func(c int) { code = c }
	defer func() { osExit = orig }()

	dest := t.TempDir()
	cmd := newInstallCmd(realAdapter)
	cmd.SetArgs([]string{
		"--runtime", "codex", "--dest", dest,
		"--index", filepath.Join(root, "index.json"),
		"--bundles", filepath.Join(root, "bundles"),
		"--catalog", filepath.Join(root, "catalog"),
		"stark-gh",
	})
	cmd.SetIn(strings.NewReader("n\n")) // decline consent
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if code != ExitConsentDeclined {
		t.Fatalf("declined consent → exit %d, want %d", code, ExitConsentDeclined)
	}
	if _, err := os.Stat(filepath.Join(dest, ".stark", "manifest-codex.json")); !os.IsNotExist(err) {
		t.Fatal("declined consent must not write a manifest")
	}
}

// Command-level: an unmanaged collision (no --force) drives RunE to exit 4.
func TestInstallCmdConflictExit4(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "index.json")); err != nil {
		t.Skipf("committed index.json absent (%v)", err)
	}
	var code int
	orig := osExit
	osExit = func(c int) { code = c }
	defer func() { osExit = orig }()

	dest := t.TempDir()
	os.WriteFile(filepath.Join(dest, "config.toml"), []byte("[mcp_servers.gh]\ncommand = 'theirs'\n"), 0o644)
	cmd := newInstallCmd(realAdapter)
	cmd.SetArgs([]string{
		"--runtime", "codex", "--dest", dest, "--yes",
		"--index", filepath.Join(root, "index.json"),
		"--bundles", filepath.Join(root, "bundles"),
		"--catalog", filepath.Join(root, "catalog"),
		"stark-gh",
	})
	cmd.Execute()
	if code != ExitConflict {
		t.Fatalf("unmanaged collision → exit %d, want %d", code, ExitConflict)
	}
}
