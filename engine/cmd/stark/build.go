package main

import (
	"fmt"

	"github.com/GetEvinced/stark-marketplace/engine/internal/build"
	"github.com/GetEvinced/stark-marketplace/engine/internal/load"
	"github.com/spf13/cobra"
)

// runBuild builds the catalog and either writes the generated tree (check=false)
// or verifies on-disk output matches (check=true). Exit codes per spec §9.8:
// 0 ok, 1 validation/build error, 2 drift.
func runBuild(catalogDir, repoRoot string, check bool) int {
	cat, err := load.Load(catalogDir)
	if err != nil {
		fmt.Println("load error:", err)
		return 1
	}
	out, err := build.Build(cat)
	if err != nil {
		fmt.Println("build error:", err)
		return 1
	}
	for _, w := range out.Warnings {
		fmt.Println("warn ", w)
	}
	fmt.Println(out.DivergenceBudget)
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
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build dist/claude + index.json + bundles/*.json (--check = drift gate)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = fix // --fix is the explicit alias for the default write behavior
			code := runBuild("catalog", ".", check)
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
	return cmd
}

// exitError carries a specific process exit code up to main.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }
func (e *exitError) ExitCode() int { return e.code }
