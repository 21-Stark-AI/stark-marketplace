// Package adapter defines the per-runtime Target interface. Each runtime target
// is independently versioned (spec §7.7) and renders a bundle into a deterministic
// set of output files keyed by `/`-separated relative paths.
package adapter

import (
	"sort"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// OutputFile is one generated file. Path uses `/` separators and is relative to
// the runtime's dist root (e.g. dist/claude/<bundle>/).
type OutputFile struct {
	Path    string
	Content []byte
}

// Finding is one non-fatal signal a target wants surfaced (emulation, divergence,
// dropped field, array foot-gun). This is the canonical contract CC-1: targets
// return a flat `[]Finding`, never a bespoke struct. Where is "<bundle>/<name>"
// (optionally "@<runtime>"); Level is "warn" or "error"; Msg is human text.
type Finding struct {
	Where string // "<bundle>/<name>" or "<bundle>/<name>@<runtime>"
	Level string // "warn" | "error"
	Msg   string
}

// Target renders a bundle's artifacts into one runtime's native shape. Targets
// iterate b.Artifacts and call merge.Resolve(a, rt) internally (CC-1); they do
// NOT receive a pre-stripped body.
type Target interface {
	Runtime() model.Runtime
	Version() string
	Render(b *model.Bundle) ([]OutputFile, []Finding, error)
}

// SortFiles orders output files by path for deterministic emission.
func SortFiles(files []OutputFile) {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
}
