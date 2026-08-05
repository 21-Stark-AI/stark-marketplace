package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/installplan"
	"github.com/21StarkCom/bifrost/engine/internal/model"
	"github.com/spf13/cobra"
)

// flagCmd builds a throwaway command carrying the two asset flags and parses argv, so
// resolveAssetFlag sees the real cobra Changed() state.
func flagCmd(t *testing.T, argv ...string) *cobra.Command {
	t.Helper()
	c := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { return nil }}
	var a, p string
	c.Flags().StringVar(&a, "assets-source", "", "")
	c.Flags().StringVar(&p, "plugin-assets", "", "")
	c.SetArgs(argv)
	if err := c.ParseFlags(argv); err != nil {
		t.Fatal(err)
	}
	return c
}

// F4/F6: unset → default; explicit "" → disabled; explicit missing dir → error;
// explicit real dir → passthrough.
func TestResolveAssetFlag(t *testing.T) {
	real := t.TempDir()

	// unset → default
	c := flagCmd(t)
	if got, err := resolveAssetFlag(c, "assets-source", "", "DEF"); err != nil || got != "DEF" {
		t.Fatalf("unset: got %q, err %v; want DEF", got, err)
	}

	// explicit empty (F6) → disabled, NOT default
	c = flagCmd(t, "--assets-source", "")
	if got, err := resolveAssetFlag(c, "assets-source", "", "DEF"); err != nil || got != "" {
		t.Fatalf("explicit empty: got %q, err %v; want disabled (\"\")", got, err)
	}

	// explicit missing dir (F4) → hard error
	c = flagCmd(t, "--assets-source", "/no/such/vendor")
	if _, err := resolveAssetFlag(c, "assets-source", "/no/such/vendor", "DEF"); err == nil {
		t.Fatal("explicit missing dir must error, not silently ship an asset-less install")
	}

	// explicit real dir → passthrough
	c = flagCmd(t, "--assets-source", real)
	if got, err := resolveAssetFlag(c, "assets-source", real, "DEF"); err != nil || got != real {
		t.Fatalf("explicit real dir: got %q, err %v; want %q", got, err, real)
	}
}

// F2: a project-local codex install (dest != $HOME) that vendored assets must warn and
// name the exports; a global install (dest == home) and a non-codex install stay quiet.
func TestWarnProjectLocalCodex(t *testing.T) {
	withAssets := &installplan.Plan{Steps: []installplan.Step{
		{Name: installplan.AssetsStepName, Bundle: "stark-gh"},
	}}

	var buf bytes.Buffer
	warnProjectLocalCodex(&buf, t.TempDir(), model.RuntimeCodex, withAssets)
	if !strings.Contains(buf.String(), "STARK_PLUGIN_ROOT") || !strings.Contains(buf.String(), "project-local") {
		t.Fatalf("project-local codex install must warn with the export: %q", buf.String())
	}

	// non-codex: silent
	buf.Reset()
	warnProjectLocalCodex(&buf, t.TempDir(), model.RuntimeClaude, withAssets)
	if buf.Len() != 0 {
		t.Fatalf("non-codex install must not warn: %q", buf.String())
	}

	// no asset step: silent
	buf.Reset()
	warnProjectLocalCodex(&buf, t.TempDir(), model.RuntimeCodex, &installplan.Plan{})
	if buf.Len() != 0 {
		t.Fatalf("no-assets install must not warn: %q", buf.String())
	}

	// dest == home: global install, the $HOME fallback is correct → silent
	buf.Reset()
	home := t.TempDir()
	t.Setenv("HOME", home)
	warnProjectLocalCodex(&buf, home, model.RuntimeCodex, withAssets)
	if buf.Len() != 0 {
		t.Fatalf("global (dest==$HOME) install must not warn: %q", buf.String())
	}
}

// F6 end-to-end: --assets-source ” --plugin-assets ” installs artifacts only — no
// vendored asset tree under .agents/stark/<bundle>/.
func TestInstallCmdEmptyFlagsDisableVendoring(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "index.json")); err != nil {
		t.Skipf("committed index.json absent (%v)", err)
	}
	orig := osExit
	osExit = func(int) {}
	defer func() { osExit = orig }()

	dest := t.TempDir()
	cmd := newInstallCmd(realAdapter)
	cmd.SetArgs([]string{
		"--runtime", "codex", "--dest", dest, "--yes",
		"--assets-source", "", "--plugin-assets", "",
		"--index", filepath.Join(root, "index.json"),
		"--bundles", filepath.Join(root, "bundles"),
		"--catalog", filepath.Join(root, "catalog"),
		"stark-gh",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".agents", "stark", "stark-gh")); err == nil {
		t.Fatal("empty asset flags must disable vendoring — no .agents/stark/stark-gh tree")
	}
}
