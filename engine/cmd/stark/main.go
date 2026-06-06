package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "stark",
		Short:         "stark-marketplace CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newValidateCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
