// Package emulate synthesizes the shape + fidelity header for emulated outputs
// (spec §6.1). Emulation shape is adapter-owned and NEVER authored via overrides.
package emulate

import "fmt"

// Header returns the generated fidelity header for an emulated artifact, wrapped
// in the target file's comment delimiters (e.g. "<!-- "/" -->" for markdown,
// "# "/"" for TOML). It is a pure function — no clock, no randomness — so output
// stays byte-stable (spec §7.6).
func Header(bundle, artifact, open, close string) string {
	const tmpl = "EMULATED from %s/%s — derived shape; may not auto-activate on this runtime; verify."
	return fmt.Sprintf("%s%s%s\n", open, fmt.Sprintf(tmpl, bundle, artifact), close)
}
