package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/GetEvinced/stark-marketplace/engine/internal/importer"
	"github.com/GetEvinced/stark-marketplace/engine/internal/load"
	"github.com/spf13/cobra"
)

// runSync regenerates the catalog artifacts (skills/commands/mcp) and the
// vendor/stark-skills/ asset snapshot from a stark-skills checkout, driven by
// each bundle's curated bundle.yaml membership manifest (skills:/commands:).
// bundle.yaml itself is preserved. With check=true it verifies the committed
// tree matches a fresh generation (drift gate, exit 2) instead of writing.
//
// Pipeline: `stark sync --from <stark-skills>` then `stark build` (which vendors
// the snapshot into dist/ and is itself drift-gated).
func runSync(from, catalogDir, repoRoot string, check bool) int {
	cat, err := load.Load(catalogDir)
	if err != nil {
		fmt.Println("load error:", err)
		return 1
	}

	expected := map[string][]byte{}            // repo-relative path -> content
	managed := []string{"vendor/stark-skills"} // trees this command fully owns

	for _, b := range cat.Bundles {
		res, err := importer.ImportForGenerator(from, b.Name, b.Skills)
		if err != nil {
			fmt.Printf("generate %s: %v\n", b.Name, err)
			return 1
		}
		// Artifacts inherit bundle-level version + runtimes (stark-skills source
		// carries neither): bump a bundle's version in bundle.yaml to publish a
		// content change (satisfies the per-artifact version-bump gate in one
		// place), and a bundle's runtimes list governs which runtimes its
		// artifacts target (e.g. stark-gh ships to claude/codex/gemini).
		for _, a := range res.Bundle.Artifacts {
			a.Version = b.Version
			if len(b.Runtimes) > 0 {
				a.Runtimes = b.Runtimes
			}
		}
		files, err := importer.ArtifactFiles(res)
		if err != nil {
			fmt.Printf("serialize %s: %v\n", b.Name, err)
			return 1
		}
		for rel, content := range files {
			expected["catalog/"+b.Name+"/"+rel] = lfNormalize(content)
		}
		// Only skills/ + commands/ are generated from stark-skills, so only those
		// are managed (stale entries pruned). mcp/ is curated in the marketplace
		// catalog (stark-skills defines no MCP servers) and left untouched.
		for _, sub := range []string{"skills", "commands"} {
			managed = append(managed, "catalog/"+b.Name+"/"+sub)
		}
		// Per-bundle plugin assets (plugins/<bundle>/tools + its own config/package.json),
		// captured into vendor/plugins/<bundle>/ and layered by `stark build` into THIS
		// bundle's dist tree only. Empty for skills-only bundles (no plugins/<bundle> dir).
		pv, err := importer.PluginVendorSnapshot(from, b.Name)
		if err != nil {
			fmt.Printf("plugin vendor %s: %v\n", b.Name, err)
			return 1
		}
		for rel, content := range pv {
			expected["vendor/plugins/"+b.Name+"/"+rel] = lfNormalize(content)
		}
		if len(pv) > 0 {
			managed = append(managed, "vendor/plugins/"+b.Name)
		}
	}

	vendor, err := importer.VendorSnapshot(from)
	if err != nil {
		fmt.Println("vendor snapshot:", err)
		return 1
	}
	for rel, content := range vendor {
		expected["vendor/stark-skills/"+rel] = lfNormalize(content)
	}

	if check {
		drift := syncDrift(repoRoot, managed, expected)
		if len(drift) > 0 {
			fmt.Println("DRIFT: committed catalog/vendor does not match a fresh sync:")
			for _, d := range drift {
				fmt.Println("  -", d)
			}
			fmt.Println("run `stark sync --from <stark-skills>` and commit the result")
			return 2
		}
		fmt.Println("OK: no drift")
		return 0
	}

	for _, r := range managed {
		_ = os.RemoveAll(filepath.Join(repoRoot, filepath.FromSlash(r)))
	}
	for _, p := range sortedStringKeys(expected) {
		abs := filepath.Join(repoRoot, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			fmt.Println("write error:", err)
			return 1
		}
		if err := os.WriteFile(abs, expected[p], 0o644); err != nil {
			fmt.Println("write error:", err)
			return 1
		}
	}
	fmt.Printf("synced %d files (catalog + vendor) from %s\n", len(expected), from)
	fmt.Println("next: `stark build` to regenerate dist/, then commit")
	return 0
}

// syncDrift returns sorted repo-relative drift paths: expected files missing or
// changed on disk, plus on-disk files under a managed root not in the expected set.
func syncDrift(repoRoot string, managed []string, expected map[string][]byte) []string {
	var drift []string
	for _, p := range sortedStringKeys(expected) {
		abs := filepath.Join(repoRoot, filepath.FromSlash(p))
		got, err := os.ReadFile(abs)
		if os.IsNotExist(err) {
			drift = append(drift, p+" (missing)")
			continue
		}
		if err != nil {
			drift = append(drift, p+" (read error: "+err.Error()+")")
			continue
		}
		if !bytes.Equal(got, expected[p]) {
			drift = append(drift, p+" (changed)")
		}
	}
	for _, r := range managed {
		base := filepath.Join(repoRoot, filepath.FromSlash(r))
		_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(repoRoot, path)
			rel = filepath.ToSlash(rel)
			if _, ok := expected[rel]; !ok {
				drift = append(drift, rel+" (unexpected)")
			}
			return nil
		})
	}
	sort.Strings(drift)
	return drift
}

func lfNormalize(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(b, []byte("\r"), []byte("\n"))
}

func sortedStringKeys(m map[string][]byte) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func newSyncCmd() *cobra.Command {
	var from string
	var check bool
	cmd := &cobra.Command{
		Use:   "sync [catalog-dir]",
		Short: "Regenerate catalog artifacts + vendor/ snapshot from a stark-skills checkout (--check = drift gate)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" {
				return fmt.Errorf("--from <stark-skills checkout> is required")
			}
			catalogDir := "catalog"
			if len(args) == 1 {
				catalogDir = args[0]
			}
			code := runSync(from, catalogDir, filepath.Dir(filepath.Clean(catalogDir)), check)
			switch code {
			case 0:
				return nil
			case 2:
				return &exitError{code: 2, msg: "drift detected"}
			default:
				return fmt.Errorf("sync failed")
			}
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "path to a stark-skills checkout (source of truth)")
	cmd.Flags().BoolVar(&check, "check", false, "verify committed catalog+vendor match a fresh sync (CI drift gate)")
	return cmd
}
