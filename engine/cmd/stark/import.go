package main

import (
	"fmt"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/importer"
	"github.com/spf13/cobra"
)

// importOpts mirrors the import flags for testability.
type importOpts struct {
	from   string
	bundle string
	dest   string // catalog dir to write under (default "catalog")
	skills string // optional comma-separated subset of skill names (empty = all)
	dryRun bool
}

// splitCSV parses a comma-separated flag into a trimmed, non-empty slice (nil when blank).
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// runImport scaffolds a canonical bundle from a stark-skills checkout (local only —
// NOT publish; spec §12). Returns an exit code per spec §9.8 (0 ok, 1 error).
func runImport(o importOpts) int {
	res, err := importer.Import(importer.Options{From: o.from, Bundle: o.bundle, Skills: splitCSV(o.skills)})
	if err != nil {
		fmt.Println("import error:", err)
		return 1
	}

	// Always print the human-metadata checklist (defaulted/guessed/dropped fields).
	fmt.Printf("Imported bundle %q: %d artifact(s)\n", res.Bundle.Name, len(res.Bundle.Artifacts))
	printChecklist(res)

	if o.dryRun {
		fmt.Printf("\n--dry-run: would write to %s/%s/ (no files written)\n", o.dest, res.Bundle.Name)
		return 0
	}
	if err := importer.WriteBundle(res, o.dest); err != nil {
		fmt.Println("write error:", err)
		return 1
	}
	fmt.Printf("\nWrote %s/%s/ — review IMPORT-NOTES.md, then run `stark validate %s`\n",
		o.dest, res.Bundle.Name, o.dest)
	return 0
}

func printChecklist(res *importer.ImportResult) {
	if len(res.Notes) == 0 {
		fmt.Println("Needs human review: none (clean import)")
		return
	}
	fmt.Println("Needs human review (defaulted/guessed/dropped):")
	for _, n := range res.Notes {
		fmt.Printf("  [ ] %s · %s — %s\n", n.Where, n.Field, n.Note)
	}
}

func newImportCmd() *cobra.Command {
	o := importOpts{dest: "catalog"}
	cmd := &cobra.Command{
		Use:   "import --from <stark-skills path> --bundle <name> [--skills a,b,c] [--dry-run]",
		Short: "Scaffold a canonical bundle from a stark-skills checkout (local; not publish)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.from == "" || o.bundle == "" {
				return fmt.Errorf("--from and --bundle are required")
			}
			if code := runImport(o); code != 0 {
				return fmt.Errorf("import failed")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&o.from, "from", "", "path to a stark-skills checkout")
	cmd.Flags().StringVar(&o.bundle, "bundle", "", "target catalog bundle name")
	cmd.Flags().StringVar(&o.skills, "skills", "", "comma-separated skill names to import (default: all skills under skill/)")
	cmd.Flags().StringVar(&o.dest, "dest", "catalog", "catalog dir to write the bundle under")
	cmd.Flags().BoolVar(&o.dryRun, "dry-run", false, "print the plan without writing")
	return cmd
}
