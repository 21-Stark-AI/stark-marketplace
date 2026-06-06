// Package build orchestrates the slice-2 pipeline: render the Claude target for
// every bundle into dist/claude/, build the lean index + per-bundle detail, and
// compute the divergence budget (spec §4.3, §7.1). Output is a path->bytes map so
// the writer/checker can diff or write deterministically.
package build

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/adapter/claude"
	"github.com/GetEvinced/stark-marketplace/engine/internal/canonjson"
	"github.com/GetEvinced/stark-marketplace/engine/internal/index"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// toLF normalizes content to LF so the in-memory set, the on-disk write, and the
// drift check all hash identical bytes regardless of any CR in source bodies
// (F-Cov#7 / spec §7.6).
func toLF(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(b, []byte("\r"), []byte("\n"))
}

// Output is the full set of generated repo-relative files plus surfaced signals.
type Output struct {
	Files            map[string][]byte // repo-relative path -> content
	Warnings         []string
	DivergenceBudget string // e.g. "diverged 0 / 2 = 0.0%"
}

// generatedRoots are the repo-relative trees this slice owns and fully regenerates.
var generatedRoots = []string{"dist/claude", "index.json", "bundles"}

// Build runs the pipeline over a loaded catalog.
func Build(cat *model.Catalog) (Output, error) {
	out := Output{Files: map[string][]byte{}}
	tgt := claude.New()

	totalArtifacts := 0
	diverged := 0
	for _, b := range cat.Bundles {
		totalArtifacts += len(b.Artifacts)
		files, findings, err := tgt.Render(b)
		if err != nil {
			return Output{}, fmt.Errorf("render %s: %w", b.Name, err)
		}
		for _, f := range files {
			out.Files["dist/claude/"+b.Name+"/"+f.Path] = toLF(f.Content)
		}
		// findings is a flat []adapter.Finding (CC-1). Surface warnings; count the
		// divergence findings (Msg prefixed "diverged:") for the budget.
		for _, fd := range findings {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%s [%s] %s", fd.Where, fd.Level, fd.Msg))
			if strings.HasPrefix(fd.Msg, "diverged:") {
				diverged++
			}
		}
	}

	// index.json + bundles/<name>.json
	idx, details := index.Build(cat)
	idxBytes, err := canonjson.Marshal(idx)
	if err != nil {
		return Output{}, err
	}
	out.Files["index.json"] = idxBytes
	names := make([]string, 0, len(details))
	for n := range details {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		db, err := canonjson.Marshal(details[n])
		if err != nil {
			return Output{}, err
		}
		out.Files["bundles/"+n+".json"] = db
	}

	pct := 0.0
	if totalArtifacts > 0 {
		pct = 100 * float64(diverged) / float64(totalArtifacts)
	}
	out.DivergenceBudget = fmt.Sprintf("diverged %d / %d = %.1f%%", diverged, totalArtifacts, pct)
	return out, nil
}

// Write cleans the generated roots under repoRoot and writes every file in out.
// Paths use `/` and are joined under repoRoot with the OS separator. Every file's
// content is normalized to LF on the way out (F-Cov#7): the determinism contract
// (spec §7.6) forbids CRLF in generated files regardless of host OS or any CR that
// slipped through a body/string value.
func Write(repoRoot string, out Output) error {
	for _, r := range generatedRoots {
		_ = os.RemoveAll(filepath.Join(repoRoot, filepath.FromSlash(r)))
	}
	paths := sortedKeys(out.Files)
	for _, p := range paths {
		abs := filepath.Join(repoRoot, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		content := bytes.ReplaceAll(out.Files[p], []byte("\r\n"), []byte("\n"))
		content = bytes.ReplaceAll(content, []byte("\r"), []byte("\n"))
		out.Files[p] = content // keep the in-memory set byte-identical to disk for Check
		if err := os.WriteFile(abs, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Check compares the expected build output against what is on disk under
// repoRoot. It returns a sorted list of drifted repo-relative paths (missing,
// changed, or unexpected-extra within the generated roots).
func Check(repoRoot string, out Output) ([]string, error) {
	var drift []string
	// expected: must exist and match
	for _, p := range sortedKeys(out.Files) {
		abs := filepath.Join(repoRoot, filepath.FromSlash(p))
		got, err := os.ReadFile(abs)
		if os.IsNotExist(err) {
			drift = append(drift, p+" (missing)")
			continue
		}
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(got, out.Files[p]) {
			drift = append(drift, p+" (changed)")
		}
	}
	// extras: any on-disk file under a generated root not in the expected set
	for _, r := range generatedRoots {
		base := filepath.Join(repoRoot, filepath.FromSlash(r))
		_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(repoRoot, path)
			rel = filepath.ToSlash(rel)
			if _, ok := out.Files[rel]; !ok {
				drift = append(drift, rel+" (unexpected)")
			}
			return nil
		})
	}
	sort.Strings(drift)
	return drift, nil
}

func sortedKeys(m map[string][]byte) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
