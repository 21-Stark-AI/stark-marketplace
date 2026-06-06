package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/GetEvinced/stark-marketplace/engine/internal/provenance"
	"github.com/spf13/cobra"
)

// runVerifyManifest verifies a signed build manifest against committed files. Exit codes
// (spec §9.8): 0 ok, 1 load/parse error, 3 integrity/digest or signature mismatch. When skipSig
// is true the cosign step is skipped (digest layer only) — used by tests and by environments
// without cosign; a warning is printed.
func runVerifyManifest(manifestPath, root string, skipSig bool) int {
	mb, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read manifest:", err)
		return 1
	}
	var m provenance.BuildManifest
	if err := json.Unmarshal(mb, &m); err != nil {
		fmt.Fprintln(os.Stderr, "parse manifest:", err)
		return 1
	}

	// 1) signature (provenance root)
	if skipSig {
		fmt.Fprintln(os.Stderr, "WARNING: signature verification skipped — digest/anti-drift check only")
	} else {
		argv := provenance.CosignVerifyCmd(manifestPath, manifestPath+".sig", manifestPath+".pem")
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "cosign verify-blob failed:", err)
			return 3
		}
	}

	// 2) digests (anti-drift)
	files := map[string][]byte{}
	for _, fd := range m.Files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(fd.Path)))
		if err != nil {
			continue // missing → reported as mismatch below
		}
		files[fd.Path] = data
	}
	if bad := provenance.VerifyDigests(&m, files); len(bad) > 0 {
		for _, p := range bad {
			fmt.Fprintf(os.Stderr, "digest mismatch: %s\n", p)
		}
		fmt.Printf("FAIL: %d digest mismatch(es)\n", len(bad))
		return 3
	}
	fmt.Printf("OK: manifest verified (%d files, %d targets)\n", len(m.Files), len(m.TargetVersions))
	return 0
}

func newVerifyManifestCmd() *cobra.Command {
	var root string
	var skipSig bool
	cmd := &cobra.Command{
		Use:   "verify-manifest <manifest.json>",
		Short: "Verify a CI-signed build manifest (cosign signature + content digests)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runVerifyManifest(args[0], root, skipSig); code != 0 {
				return fmt.Errorf("verification failed")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "repo root the manifest paths are relative to")
	cmd.Flags().BoolVar(&skipSig, "skip-signature", false, "skip cosign (digest/anti-drift only)")
	return cmd
}
