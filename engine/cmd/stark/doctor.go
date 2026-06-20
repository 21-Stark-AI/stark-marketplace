package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/install"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var dest, runtime string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check the runtime env (Node) + verify installed manifests still match",
		RunE: func(c *cobra.Command, args []string) error {
			var ok, broken []string
			var emulated []string

			// Environment: vendored skill tools run via
			// `node --experimental-strip-types` (Node >= 22.6). Checked always,
			// even with no installed manifest.
			if okMsg, brokenMsg := nodeVersionCheck(); brokenMsg != "" {
				broken = append(broken, brokenMsg)
			} else {
				ok = append(ok, okMsg)
			}

			// Installed-manifest integrity. A missing manifest is a soft note
			// (env-only doctor is valid), not a hard failure.
			mPath := filepath.Join(dest, ".stark", "manifest-"+runtime+".json")
			if rep, err := install.Doctor(dest, mPath); err != nil {
				ok = append(ok, fmt.Sprintf("manifest %s not audited (%v)", runtime, err))
			} else {
				ok = append(ok, rep.OK...)
				broken = append(broken, rep.Broken...)
				emulated = append(emulated, rep.Emulated...)
			}

			code := ExitOK
			if len(broken) > 0 {
				code = ExitDrift
			}
			if jsonOut {
				emitJSON(os.Stdout, "doctor", code, map[string]any{
					"ok": ok, "broken": broken, "emulated": emulated})
			} else {
				for _, m := range ok {
					fmt.Printf("ok      %s\n", m)
				}
				for _, b := range broken {
					fmt.Printf("BROKEN  %s\n", b)
				}
			}
			if code != ExitOK {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dest, "dest", ".", "destination root to audit")
	cmd.Flags().StringVar(&runtime, "runtime", "codex", "runtime manifest to audit")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "machine-readable output")
	return cmd
}

// minNodeMajor / minNodeMinor is the floor for native TypeScript stripping
// (`node --experimental-strip-types`), which every vendored skill tool needs.
const (
	minNodeMajor = 22
	minNodeMinor = 6
)

// nodeVersionCheck verifies Node is on PATH and >= 22.6. Exactly one of the two
// return values is non-empty.
func nodeVersionCheck() (okMsg, brokenMsg string) {
	out, err := exec.Command("node", "--version").Output()
	if err != nil {
		return "", fmt.Sprintf("node: not found on PATH — skills cannot run (need >= %d.%d)", minNodeMajor, minNodeMinor)
	}
	v := strings.TrimSpace(string(out)) // e.g. "v22.6.0"
	maj, min, parsed := parseNodeVersion(v)
	if !parsed {
		return "", fmt.Sprintf("node: unparseable version %q (need >= %d.%d)", v, minNodeMajor, minNodeMinor)
	}
	if maj < minNodeMajor || (maj == minNodeMajor && min < minNodeMinor) {
		return "", fmt.Sprintf("node %s too old — need >= %d.%d for --experimental-strip-types", v, minNodeMajor, minNodeMinor)
	}
	return fmt.Sprintf("node %s (>= %d.%d, supports --experimental-strip-types)", v, minNodeMajor, minNodeMinor), ""
}

func parseNodeVersion(v string) (maj, min int, ok bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	return maj, min, err1 == nil && err2 == nil
}
