package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/registry"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/build"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/load"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/provenance"
	"github.com/spf13/cobra"
)

// runBuild builds the catalog and either writes the generated tree (check=false)
// or verifies on-disk output matches (check=true). Exit codes per spec §9.8:
// 0 ok, 1 validation/build error, 2 drift. When manifestPath is non-empty, the build manifest
// (adapter target versions + content digests over the generated files) is written there — the
// blob the sign-manifest workflow cosign-signs (spec §7.5).
func runBuild(catalogDir, repoRoot, manifestPath, assetsSource string, check bool) int {
	cat, err := load.Load(catalogDir)
	if err != nil {
		fmt.Println("load error:", err)
		return 1
	}
	// Default the vendor snapshot to <repoRoot>/vendor/stark-skills when present,
	// so committed builds are self-contained without an explicit flag.
	if assetsSource == "" {
		def := filepath.Join(repoRoot, "vendor", "stark-skills")
		if fi, statErr := os.Stat(def); statErr == nil && fi.IsDir() {
			assetsSource = def
		}
	}
	// Per-bundle plugin assets live under <repoRoot>/vendor/plugins/<bundle>
	// (written by `stark sync`); default to it when present so the committed
	// build layers each bundle's own plugin tools/config without an explicit flag.
	pluginAssetsRoot := ""
	if def := filepath.Join(repoRoot, "vendor", "plugins"); dirExists(def) {
		pluginAssetsRoot = def
	}
	out, err := build.Build(cat, build.Options{AssetsSource: assetsSource, PluginAssetsRoot: pluginAssetsRoot})
	if err != nil {
		fmt.Println("build error:", err)
		return 1
	}
	if assetsSource != "" {
		fmt.Println("vendored assets from", assetsSource)
	}
	for _, w := range out.Warnings {
		fmt.Println("warn ", w)
	}
	fmt.Println(out.DivergenceBudget)
	if manifestPath != "" {
		m := provenance.Compute(targetVersionsFromRegistry(), out.Files)
		mb, err := m.Marshal()
		if err != nil {
			fmt.Println("manifest error:", err)
			return 1
		}
		if err := os.WriteFile(manifestPath, append(mb, '\n'), 0o644); err != nil {
			fmt.Println("manifest write error:", err)
			return 1
		}
		fmt.Printf("wrote build manifest: %s (%d files, %d targets)\n", manifestPath, len(m.Files), len(m.TargetVersions))
	}
	if check {
		drift, err := build.Check(repoRoot, out)
		if err != nil {
			fmt.Println("check error:", err)
			return 1
		}
		if len(drift) > 0 {
			fmt.Println("DRIFT: committed output does not match a fresh build:")
			for _, d := range drift {
				fmt.Println("  -", d)
			}
			fmt.Println("run `stark build --fix` and commit the result")
			return 2
		}
		fmt.Println("OK: no drift")
		return 0
	}
	if err := build.Write(repoRoot, out); err != nil {
		fmt.Println("write error:", err)
		return 1
	}
	fmt.Printf("wrote %d generated files\n", len(out.Files))
	return 0
}

func newBuildCmd() *cobra.Command {
	var check, fix bool
	var manifest, assetsSource string
	cmd := &cobra.Command{
		Use:   "build [catalog-dir]",
		Short: "Build dist/claude + index.json + bundles/*.json (--check = drift gate)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = fix // --fix is the explicit alias for the default write behavior
			// repoRoot (where generated output lives) is the catalog dir's parent, so
			// `build [../catalog]` works from any CWD — e.g. CI runs it from engine/.
			// Clean first so a trailing slash (`catalog/`, common from tab-completion)
			// doesn't make Dir return the catalog dir itself instead of its parent.
			catalogDir := "catalog"
			if len(args) == 1 {
				catalogDir = args[0]
			}
			code := runBuild(catalogDir, filepath.Dir(filepath.Clean(catalogDir)), manifest, assetsSource, check)
			switch code {
			case 0:
				return nil
			case 2:
				return &exitError{code: 2, msg: "drift detected"}
			default:
				return fmt.Errorf("build failed")
			}
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "verify committed output matches a fresh build (CI drift gate)")
	cmd.Flags().BoolVar(&fix, "fix", false, "regenerate committed output (default behavior)")
	cmd.Flags().StringVar(&manifest, "manifest", "", "also write a signable build manifest (target versions + digests) to this path")
	cmd.Flags().StringVar(&assetsSource, "assets-source", "", "directory vendored verbatim into each plugin (default: <repo>/vendor/stark-skills if present)")
	return cmd
}

// targetVersionsFromRegistry derives runtime→adapter-version from the registry. Each target's
// Version() is "<runtime>@<n>" (e.g. "claude@1"); the manifest records the integer n.
func targetVersionsFromRegistry() map[string]int {
	out := map[string]int{}
	for rt, tgt := range registry.All() {
		v := tgt.Version()
		if i := strings.LastIndexByte(v, '@'); i >= 0 {
			if n, err := strconv.Atoi(v[i+1:]); err == nil {
				out[string(rt)] = n
			}
		}
	}
	return out
}

// exitError carries a specific process exit code up to main.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }
func (e *exitError) ExitCode() int { return e.code }

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
