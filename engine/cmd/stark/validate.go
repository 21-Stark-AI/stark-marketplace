package main

import (
	"fmt"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/load"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/validate"
	"github.com/spf13/cobra"
)

// runValidate loads + validates a catalog dir, prints findings, returns an exit code.
// Exit codes per spec §9.8: 0 ok, 1 validation error.
func runValidate(catalogDir string) int {
	cat, err := load.Load(catalogDir)
	if err != nil {
		fmt.Println("load error:", err)
		return 1
	}
	r := validate.Catalog(cat)
	for _, w := range r.Warnings {
		fmt.Printf("warn  %s: %s\n", w.Where, w.Msg)
	}
	for _, e := range r.Errors {
		fmt.Printf("error %s: %s\n", e.Where, e.Msg)
	}
	if r.HasErrors() {
		fmt.Printf("FAIL: %d error(s), %d warning(s)\n", len(r.Errors), len(r.Warnings))
		return 1
	}
	fmt.Printf("OK: %d warning(s)\n", len(r.Warnings))
	return 0
}

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [catalog-dir]",
		Short: "Validate the canonical catalog",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "catalog"
			if len(args) == 1 {
				dir = args[0]
			}
			if code := runValidate(dir); code != 0 {
				return fmt.Errorf("validation failed")
			}
			return nil
		},
	}
}
