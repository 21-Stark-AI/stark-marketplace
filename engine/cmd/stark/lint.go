package main

import (
	"fmt"

	"github.com/GetEvinced/stark-marketplace/engine/internal/load"
	"github.com/GetEvinced/stark-marketplace/engine/internal/validate"
	"github.com/spf13/cobra"
)

// runLint loads a catalog and prints suspicious-pattern findings. It is informational: it always
// returns 0 (never blocks CI) but prints a machine-readable summary line so PR output surfaces
// the count (spec §7.4 / §14).
func runLint(catalogDir string) int {
	cat, err := load.Load(catalogDir)
	if err != nil {
		fmt.Println("load error:", err)
		return 0
	}
	r := validate.LintBodies(cat)
	for _, w := range r.Warnings {
		fmt.Printf("lint  %s: %s\n", w.Where, w.Msg)
	}
	fmt.Printf("LINT-SUMMARY: %d suspicious-pattern finding(s)\n", len(r.Warnings))
	return 0
}

func newLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint [catalog-dir]",
		Short: "Informational content scan of artifact bodies (suspicious patterns)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "catalog"
			if len(args) == 1 {
				dir = args[0]
			}
			runLint(dir)
			return nil
		},
	}
}
