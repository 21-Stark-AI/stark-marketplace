package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/indexio"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
	"github.com/spf13/cobra"
)

type searchOpts struct {
	query             string
	typ               string
	tag               string
	runtime           model.Runtime
	maturity          string
	includeDeprecated bool
}

// searchIndex filters + sorts entries deterministically (bundle, then name).
func searchIndex(idx *indexio.Index, o searchOpts) []indexio.Entry {
	q := strings.ToLower(o.query)
	var out []indexio.Entry
	for _, e := range idx.Artifacts {
		if e.Maturity == model.MaturityDeprecated && !o.includeDeprecated {
			continue
		}
		if o.typ != "" && string(e.Type) != o.typ {
			continue
		}
		if o.maturity != "" && string(e.Maturity) != o.maturity {
			continue
		}
		if o.runtime != "" {
			if lvl, ok := e.Support[o.runtime]; !ok || lvl == model.SupportUnsupported {
				continue
			}
		}
		if o.tag != "" && !containsStr(e.Tags, o.tag) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(e.Name), q) &&
			!strings.Contains(strings.ToLower(e.Bundle), q) && !tagMatch(e.Tags, q) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bundle != out[j].Bundle {
			return out[i].Bundle < out[j].Bundle
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func tagMatch(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func newSearchCmd() *cobra.Command {
	var o searchOpts
	var rt, indexPath string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search the lean index",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 1 {
				o.query = args[0]
			}
			if rt != "" {
				r, err := model.ParseRuntime(rt)
				if err != nil {
					return err
				}
				o.runtime = r
			}
			idx, err := indexio.LoadIndex(indexPath)
			if err != nil {
				os.Exit(indexLoadExit(err))
			}
			res := searchIndex(idx, o)
			if jsonOut {
				rows := make([]map[string]any, 0, len(res))
				for _, e := range res {
					rows = append(rows, map[string]any{"name": e.Name, "type": e.Type,
						"bundle": e.Bundle, "version": e.Version, "support": e.Support})
				}
				emitJSON(os.Stdout, "search", ExitOK, map[string]any{"results": rows})
				return nil
			}
			for _, e := range res {
				fmt.Printf("%-20s %-8s %-16s v%s\n", e.Name, e.Type, e.Bundle, e.Version)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&o.typ, "type", "", "filter by artifact type")
	cmd.Flags().StringVar(&o.tag, "tag", "", "filter by tag")
	cmd.Flags().StringVar(&rt, "runtime", "", "filter to a supported runtime")
	cmd.Flags().StringVar(&o.maturity, "maturity", "", "filter by maturity")
	cmd.Flags().BoolVar(&o.includeDeprecated, "include-deprecated", false, "include deprecated artifacts")
	cmd.Flags().StringVar(&indexPath, "index", "index.json", "path to index.json")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "machine-readable output")
	return cmd
}

// indexLoadExit maps an index load error to the right exit code (schemaVersion → 5).
func indexLoadExit(err error) int {
	fmt.Fprintln(os.Stderr, "error:", err)
	if strings.Contains(err.Error(), "schemaVersion") {
		return ExitSchemaVersion
	}
	return ExitValidation
}
