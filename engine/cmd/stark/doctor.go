package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/GetEvinced/stark-marketplace/engine/internal/install"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var dest, runtime string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Verify installed manifests still match (files, merged keys, sentinels)",
		RunE: func(c *cobra.Command, args []string) error {
			mPath := filepath.Join(dest, ".stark", "manifest-"+runtime+".json")
			rep, err := install.Doctor(dest, mPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "doctor:", err)
				os.Exit(ExitValidation)
			}
			if jsonOut {
				code := ExitOK
				if len(rep.Broken) > 0 {
					code = ExitDrift
				}
				emitJSON(os.Stdout, "doctor", code, map[string]any{
					"ok": rep.OK, "broken": rep.Broken, "emulated": rep.Emulated})
			} else {
				for _, ok := range rep.OK {
					fmt.Printf("ok      %s\n", ok)
				}
				for _, b := range rep.Broken {
					fmt.Printf("BROKEN  %s\n", b)
				}
			}
			if len(rep.Broken) > 0 {
				os.Exit(ExitDrift)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dest, "dest", ".", "destination root to audit")
	cmd.Flags().StringVar(&runtime, "runtime", "codex", "runtime manifest to audit")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "machine-readable output")
	return cmd
}
