package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/GetEvinced/stark-marketplace/engine/internal/bumps"
	"github.com/GetEvinced/stark-marketplace/engine/internal/digest"
	"github.com/GetEvinced/stark-marketplace/engine/internal/index"
	"github.com/GetEvinced/stark-marketplace/engine/internal/load"
	"github.com/spf13/cobra"
)

// prevIndexJSON returns the previously committed index.json bytes, preferring
// origin/main and falling back to HEAD. Returns nil (skip) when neither ref has
// the file (first commit / fresh repo).
func prevIndexJSON(repoRoot string) []byte {
	for _, ref := range []string{"origin/main:index.json", "HEAD:index.json"} {
		cmd := exec.Command("git", "-C", repoRoot, "show", ref)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err == nil {
			return stdout.Bytes()
		}
	}
	return nil
}

// leanPrev is the minimal shape we read from a previous index.json (CC-2 keys).
type leanPrev struct {
	Artifacts []struct {
		Name    string `json:"name"`
		Bundle  string `json:"bundle"`
		Version string `json:"version"`
		Digest  string `json:"digest"`
	} `json:"artifacts"`
}

// runCheckBumps loads the previous committed index + the current catalog and
// errors (exit 1) on any version-bump immutability violation (CC-5 / spec §11).
func runCheckBumps(catalogDir, repoRoot string) int {
	prevBytes := prevIndexJSON(repoRoot)
	prev := map[string]bumps.Previous{}
	if len(prevBytes) > 0 {
		var lp leanPrev
		if err := json.Unmarshal(prevBytes, &lp); err != nil {
			fmt.Println("check-bumps: cannot parse previous index.json:", err)
			return 1
		}
		for _, e := range lp.Artifacts {
			prev[e.Bundle+"/"+e.Name] = bumps.Previous{Version: e.Version, Digest: e.Digest}
		}
	}

	cat, err := load.Load(catalogDir)
	if err != nil {
		fmt.Println("check-bumps: load error:", err)
		return 1
	}
	cur := map[string]bumps.Current{}
	for _, b := range cat.Bundles {
		for _, a := range b.Artifacts {
			cur[b.Name+"/"+a.Name] = bumps.Current{Version: a.Version, SourceDigest: digest.Source(a)}
		}
	}
	_ = index.SchemaVersion // keep digest/index contract in one place (CC-2/CC-5)

	violations := bumps.Check(prev, cur)
	if len(violations) == 0 {
		fmt.Println("OK: no un-bumped source changes")
		return 0
	}
	fmt.Println("VERSION-BUMP GATE: canonical source changed without a version bump:")
	for _, v := range violations {
		fmt.Printf("  - %s (version still %s)\n    old %s\n    new %s\n", v.Key, v.Version, v.OldDigest, v.NewDigest)
	}
	fmt.Println("bump the artifact's `version` (semver) and rebuild")
	return 1
}

func newCheckBumpsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check-bumps",
		Short: "Fail if an artifact's canonical source changed without a version bump (spec §11)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runCheckBumps("catalog", "."); code != 0 {
				return fmt.Errorf("version-bump gate failed")
			}
			return nil
		},
	}
}
