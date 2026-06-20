package main

import (
	"fmt"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/load"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/validate"
	"github.com/spf13/cobra"
)

// runLint loads a catalog and prints suspicious-pattern findings. By default it is
// informational and returns 0 — preserving the spec §7.4 surfacing-only contract for
// callers that opt out of blocking. Pass strict=true to return a non-zero exit when
// any finding lands; CI uses this to fail-closed on the dangerous patterns
// (curl-pipe-shell, prompt-injection, secret-file-read, base64-blob). Always prints
// LINT-SUMMARY so PR output surfaces the count.
func runLint(catalogDir string, strict bool) int {
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
	if strict && len(r.Warnings) > 0 {
		return 2
	}
	return 0
}

func newLintCmd() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "lint [catalog-dir]",
		Short: "Content scan of artifact bodies (suspicious patterns); use --strict to fail on findings",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "catalog"
			if len(args) == 1 {
				dir = args[0]
			}
			if code := runLint(dir, strict); code != 0 {
				return fmt.Errorf("lint failed (strict)")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "exit non-zero on any finding (CI gate)")
	return cmd
}
