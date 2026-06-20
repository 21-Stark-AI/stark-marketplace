package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/indexio"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/installplan"
	"github.com/spf13/cobra"
)

// renderInfo builds the info text + exit code. ref splits "bundle" vs "bundle/artifact".
func renderInfo(indexPath, bundlesDir, bundle, artifact string) (string, int) {
	idx, err := indexio.LoadIndex(indexPath)
	if err != nil {
		return err.Error(), indexLoadExit(err)
	}
	detail, err := indexio.LoadBundleDetail(bundlesDir, bundle)
	if err != nil {
		return err.Error(), ExitValidation
	}
	var sb strings.Builder
	// bundle metadata comes from the detail's "bundle" object (CC-3) — there is no
	// bundle-type row in the lean index.
	fmt.Fprintf(&sb, "bundle %s v%s — %s\n", detail.Bundle.Name, detail.Bundle.Version, detail.Bundle.Description)
	for _, a := range detail.Artifacts {
		if artifact != "" && a.Name != artifact {
			continue
		}
		fmt.Fprintf(&sb, "  %s/%s (%s) v%s\n", detail.Bundle.Name, a.Name, a.Type, a.Version)
		var rts []string
		for rt, lvl := range a.Support {
			rts = append(rts, fmt.Sprintf("%s=%s", rt, lvl))
		}
		sort.Strings(rts)
		fmt.Fprintf(&sb, "    support: %s\n", strings.Join(rts, " "))
		var notes []string
		for rt, n := range a.FidelityNotes {
			if n != "" {
				notes = append(notes, fmt.Sprintf("%s: %s", rt, n))
			}
		}
		sort.Strings(notes)
		if len(notes) > 0 {
			fmt.Fprintf(&sb, "    fidelity: %s\n", strings.Join(notes, "; "))
		}
		// dependency closure (resolved over the whole index/detail set)
		closure, err := installplan.ClosureRefs(idx, bundlesDir, detail.Bundle.Name, a.Name, a.Type)
		if err == nil && len(closure) > 0 {
			fmt.Fprintf(&sb, "    requires (closure): %s\n", strings.Join(closure, ", "))
		}
	}
	return sb.String(), ExitOK
}

func newInfoCmd() *cobra.Command {
	var indexPath, bundlesDir string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "info <bundle[/artifact]>",
		Short: "Show metadata, support matrix, dependency closure",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			bundle, artifact := splitRef(args[0])
			out, code := renderInfo(indexPath, bundlesDir, bundle, artifact)
			if jsonOut {
				emitJSON(os.Stdout, "info", code, map[string]any{"text": out})
			} else {
				fmt.Print(out)
			}
			if code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&indexPath, "index", "index.json", "path to index.json")
	cmd.Flags().StringVar(&bundlesDir, "bundles", "bundles", "path to bundles/ dir")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "machine-readable output")
	return cmd
}

// splitRef parses "bundle" or "bundle/artifact".
func splitRef(ref string) (bundle, artifact string) {
	if i := strings.IndexByte(ref, '/'); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return ref, ""
}
