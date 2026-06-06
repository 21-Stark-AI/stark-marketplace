// Package importer scaffolds a canonical catalog bundle from an existing stark-skills
// checkout (spec §12). It is LOCAL scaffolding only — it never publishes. It maps known
// source frontmatter to the plan-01 canonical superset, defaults the rest, and records every
// defaulted/guessed/dropped field as a human-review note.
package importer

import (
	"bytes"
	"errors"
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// MetaNote records one field that was defaulted, guessed, or dropped during import,
// so a human can review it. Surfaced in the printed checklist + IMPORT-NOTES.md.
type MetaNote struct {
	Where string // "<bundle>/<type>/<name>" or "<bundle>/bundle.yaml"
	Field string // canonical field name affected
	Note  string // human-readable explanation of what to verify
}

// ImportResult is the outcome of an import: the in-memory bundle plus the
// human-metadata checklist. The CLI serializes Bundle and prints/writes Notes.
type ImportResult struct {
	Bundle *model.Bundle
	Notes  []MetaNote
}

func (r *ImportResult) note(where, field, note string) {
	r.Notes = append(r.Notes, MetaNote{Where: where, Field: field, Note: note})
}

// Options configures a single bundle-by-bundle import (spec §12).
type Options struct {
	From   string   // path to a stark-skills checkout
	Bundle string   // target catalog bundle name
	Skills []string // optional: import ONLY these skill names (nil/empty = every skill under skill/)
}

// Import reads a stark-skills checkout and produces ONE canonical bundle in memory
// plus a human-metadata checklist. Pure: no writes, no network (the CLI does I/O).
// Source selection is bundle-by-bundle: skills are taken from skill/ (all, or the
// Skills subset), and a same-named plugin is taken from plugins/<bundle> when present.
func Import(opts Options) (*ImportResult, error) {
	if opts.Bundle == "" {
		return nil, errors.New("import: --bundle is required")
	}
	res := &ImportResult{Bundle: newBundle(opts.Bundle)}
	if err := importSkills(opts.From, opts.Bundle, opts.Skills, res); err != nil {
		return nil, err
	}
	if err := importPlugin(opts.From, opts.Bundle, res); err != nil {
		return nil, err
	}
	if len(res.Bundle.Artifacts) == 0 {
		return nil, errors.New("import: nothing to import (no matching skill/ entries or plugins/" + opts.Bundle + " under --from)")
	}
	finalizeBundle(res)
	return res, nil
}

// newBundle creates a defaulted bundle shell; bundle-level notes are added in finalize.
func newBundle(name string) *model.Bundle {
	return &model.Bundle{
		Name:     name,
		Version:  defaultVersion,
		Owner:    model.Owner{Name: defaultOwnerName, Email: defaultOwnerEmail},
		Maturity: defaultMaturity,
		Runtimes: defaultRuntimes(),
	}
}

// finalizeBundle fills bundle-level metadata that has no source and records notes.
func finalizeBundle(res *ImportResult) {
	b := res.Bundle
	where := b.Name + "/bundle.yaml"
	if b.Description == "" {
		b.Description = b.Name + " (imported from stark-skills)"
		res.note(where, "description", "placeholder description; write a real one")
	}
	if b.Category == "" {
		res.note(where, "category", "no bundle category; assign one")
	}
	res.note(where, "version", "bundle version defaulted to "+defaultVersion+"; set the real semver")
}

var fmDelim = []byte("---\n")

// splitFrontmatter mirrors load.splitFrontmatter (kept local to avoid exporting it).
func splitFrontmatter(data []byte) (fm []byte, body string, err error) {
	if !bytes.HasPrefix(data, fmDelim) {
		return nil, "", errors.New("missing frontmatter: file must start with '---'")
	}
	rest := data[len(fmDelim):]
	end := bytes.Index(rest, fmDelim)
	if end < 0 {
		return nil, "", errors.New("unterminated frontmatter: missing closing '---'")
	}
	return rest[:end], string(rest[end+len(fmDelim):]), nil
}

func normalizeLF(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

// cleanBody drops the single conventional blank line between the frontmatter's closing `---`
// and the markdown content, so the canonical Body starts at the content (and round-trips: the
// serializer re-emits `---\n…---\n<body>`). A body authored with no blank-line separator is a
// no-op.
func cleanBody(body string) string {
	return strings.TrimPrefix(body, "\n")
}
