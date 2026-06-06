package main

import (
	"fmt"
	"os"

	"github.com/GetEvinced/stark-marketplace/engine/internal/indexio"
	"github.com/spf13/cobra"
)

// Version is overridden at release time via -ldflags "-X main.Version=...".
var Version = "dev"

func versionString() string {
	return fmt.Sprintf("stark %s (index schemaVersion %d..%d)",
		Version, indexio.SchemaVersionMin, indexio.SchemaVersionMax)
}

// checkIndexSupported maps an index schemaVersion to an exit code (spec §9.7).
func checkIndexSupported(v int) int {
	if err := indexio.AssertSchemaVersion(v); err != nil {
		return ExitSchemaVersion
	}
	return ExitOK
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version + supported index schemaVersion range",
		Run:   func(c *cobra.Command, args []string) { fmt.Println(versionString()) },
	}
}

func newSelfUpdateCmd() *cobra.Command {
	var indexPath string
	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update the stark binary (asserts supported schemaVersion range)",
		RunE: func(c *cobra.Command, args []string) error {
			if indexPath != "" {
				idx, err := indexio.LoadIndex(indexPath)
				if err != nil {
					// Only an out-of-range schemaVersion is exit 5; an IO/parse failure is a
					// validation error (exit 1). indexLoadExit discriminates, same as the other
					// commands — self-update must not misreport every load failure as exit 5.
					fmt.Fprintln(os.Stderr, err)
					osExit(indexLoadExit(err))
					return err
				}
				if code := checkIndexSupported(idx.SchemaVersion); code != ExitOK {
					osExit(code)
					return nil
				}
			}
			fmt.Println(versionString())
			fmt.Println("self-update: download signed release binary from GitHub Releases (see docs §9.7)")
			return nil
		},
	}
	cmd.Flags().StringVar(&indexPath, "index", "", "optional index to assert compatibility")
	return cmd
}
