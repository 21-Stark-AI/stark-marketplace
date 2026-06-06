// Package marketplace projects the catalog into the native Claude Code
// marketplace manifest (dist/claude/.claude-plugin/marketplace.json).
//
// CRITICAL CONTRACT (spec §8, red-team Part B):
//   - The manifest ROOT uses `owner` (name/email).
//   - Each plugins[] ENTRY uses `author` (NOT owner), plus source/version/
//     description/category/tags/strict.
//
// These two keys are deliberately distinct types/fields so the distinction
// cannot rot.
package marketplace

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// ManifestRelPath is the manifest's location relative to the Claude dist root
// (spec §5). CC reads dist/claude/.claude-plugin/marketplace.json from the repo.
const ManifestRelPath = ".claude-plugin/marketplace.json"

// Owner identifies a maintainer (root `owner`) or plugin author (entry `author`).
type Owner struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// Source locates a plugin's tree. It marshals as a bare string when Path is set
// (the same-repo relative form, resolved relative to the marketplace root — the
// directory containing .claude-plugin/), otherwise as the native CC discriminated
// object form keyed by `source` (github|url|git-subdir|npm). Exactly one form is
// emitted per source.
type Source struct {
	Path string `json:"-"` // string form: relative path from the marketplace root, e.g. ./dist/claude/stark-gh

	// object form (set Type to one of github|url|git-subdir|npm):
	Type     string `json:"-"`
	Repo     string `json:"-"` // github: "owner/repo"
	URL      string `json:"-"` // url / git-subdir
	SubPath  string `json:"-"` // git-subdir: path within the repo
	Package  string `json:"-"` // npm
	Version  string `json:"-"` // npm (optional)
	Registry string `json:"-"` // npm (optional)
	Ref      string `json:"-"` // optional branch/tag
	SHA      string `json:"-"` // optional commit sha
}

// sourceObj is the CC discriminated object-source shape (struct field order is
// preserved by encoding/json → deterministic).
type sourceObj struct {
	Source   string `json:"source"`
	Repo     string `json:"repo,omitempty"`
	URL      string `json:"url,omitempty"`
	Path     string `json:"path,omitempty"`
	Package  string `json:"package,omitempty"`
	Version  string `json:"version,omitempty"`
	Registry string `json:"registry,omitempty"`
	Ref      string `json:"ref,omitempty"`
	SHA      string `json:"sha,omitempty"`
}

// MarshalJSON emits the string form when Path is set, else the discriminated
// object form per the CC marketplace schema.
func (s Source) MarshalJSON() ([]byte, error) {
	if s.Path != "" {
		return json.Marshal(s.Path)
	}
	return json.Marshal(sourceObj{
		Source:   s.Type,
		Repo:     s.Repo,
		URL:      s.URL,
		Path:     s.SubPath,
		Package:  s.Package,
		Version:  s.Version,
		Registry: s.Registry,
		Ref:      s.Ref,
		SHA:      s.SHA,
	})
}

// Plugin is one plugins[] entry — exactly one bundle. Uses `author`, not `owner`.
type Plugin struct {
	Name        string   `json:"name"`
	Source      Source   `json:"source"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version"`
	Author      Owner    `json:"author"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Strict      bool     `json:"strict"`
}

// Manifest is the whole .claude-plugin/marketplace.json. Root uses `owner`.
type Manifest struct {
	Name    string   `json:"name"`
	Owner   Owner    `json:"owner"`
	Plugins []Plugin `json:"plugins"`
}

// Options configures manifest generation. Pure inputs only — no clock/env.
type Options struct {
	Name     string // marketplace name, e.g. "stark-marketplace"
	Owner    Owner  // ROOT owner
	DistRoot string // relative path to the committed claude dist, e.g. "./dist/claude"
}

// Generate projects a loaded catalog into the native CC manifest. One plugins[]
// entry per bundle, sorted by bundle name for determinism (spec §7.6). The
// per-bundle source points at the committed dist/claude/<bundle>/ tree.
func Generate(cat *model.Catalog, opts Options) Manifest {
	bundles := append([]*model.Bundle(nil), cat.Bundles...)
	sort.Slice(bundles, func(i, j int) bool { return bundles[i].Name < bundles[j].Name })

	m := Manifest{Name: opts.Name, Owner: opts.Owner}
	for _, b := range bundles {
		m.Plugins = append(m.Plugins, Plugin{
			Name:        b.Name,
			Source:      Source{Path: opts.DistRoot + "/" + b.Name}, // keep ./ prefix + / separators (§7.6)
			Description: b.Description,
			Version:     b.Version,
			Author:      Owner{Name: b.Owner.Name, Email: b.Owner.Email},
			Category:    b.Category,
			Tags:        append([]string(nil), b.Tags...),
			Strict:      true,
		})
	}
	return m
}

// Marshal serializes a manifest deterministically: 2-space indent, no HTML
// escaping, LF line endings, single trailing newline. This is THE canonical
// encoder for the golden test and the committed dist file.
func Marshal(m Manifest) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil { // Encode appends a trailing newline
		return nil, err
	}
	return buf.Bytes(), nil
}
