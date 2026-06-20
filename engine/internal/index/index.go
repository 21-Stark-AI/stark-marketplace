// Package index projects a loaded catalog into the lean search index
// (index.json) plus per-bundle detail files (bundles/<name>.json), per spec §7.5.
// The lean index carries only what search needs; consumers ignore unknown fields.
package index

import (
	"fmt"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/capability"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/registry"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/digest"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/merge"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// SchemaVersion is the index schema version (spec §7.5 N-1 compat).
const SchemaVersion = 1

// Entry is one lean index row (CC-2). Top-level key is `artifacts`; description is
// carried so search can show it without fetching bundle detail.
type Entry struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Bundle      string            `json:"bundle"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Category    string            `json:"category,omitempty"`
	Maturity    string            `json:"maturity,omitempty"`
	Version     string            `json:"version"`
	Runtimes    []string          `json:"runtimes"` // CC-2: search filters by runtime without fetching detail
	Support     map[string]string `json:"support"`  // runtime -> native|emulated|unsupported
	Digest      string            `json:"digest"`
}

// GeneratedBy is the CC-2 index-level provenance hook: the adapter target versions
// that produced this index (spec §7.7). Lets consumers detect adapter-version skew.
type GeneratedBy struct {
	AdapterVersions map[string]string `json:"adapterVersions"`
}

// Index is the lean search index.
type Index struct {
	SchemaVersion int         `json:"schemaVersion"`
	GeneratedBy   GeneratedBy `json:"generatedBy"`
	Artifacts     []Entry     `json:"artifacts"`
}

// BundleDetail is the full per-bundle detail file (CC-3 structured shape).
type BundleDetail struct {
	SchemaVersion int           `json:"schemaVersion"`
	Bundle        BundleMeta    `json:"bundle"`
	Artifacts     []DetailEntry `json:"artifacts"`
}

// BundleMeta is the display metadata for a bundle (CC-3).
type BundleMeta struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Owner       string   `json:"owner,omitempty"`
	Maturity    string   `json:"maturity,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
}

// DetailEntry is one artifact in the CC-3 detail shape.
type DetailEntry struct {
	Name          string              `json:"name"`
	Type          string              `json:"type"`
	Description   string              `json:"description,omitempty"`
	Version       string              `json:"version"`
	Runtimes      []string            `json:"runtimes"`
	Support       map[string]string   `json:"support"`         // runtime -> native|emulated|unsupported
	Tools         []string            `json:"tools,omitempty"` // agent tool grants, surfaced for the install-consent UX (spec §7.4/§9.3)
	Requires      []Requirement       `json:"requires"`        // [] not null
	Diverged      bool                `json:"diverged"`
	Outputs       map[string][]Output `json:"outputs"`       // runtime -> emitted files
	FidelityNotes map[string]string   `json:"fidelityNotes"` // runtime -> note (emulated only)
}

// Requirement is a dependency reference (CC-3).
type Requirement struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
}

// Output describes one generated file for a runtime (CC-3). kind ∈
// file | mergeJSONKey | mergeTOMLKey | sentinel.
type Output struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Key      string `json:"key,omitempty"`      // for merge* kinds
	Sentinel string `json:"sentinel,omitempty"` // for sentinel kind
	Emulated bool   `json:"emulated"`
}

// supportFor returns the capability badge for (type, runtime) from the versioned
// matrix (§6) — native | emulated | unsupported (CC-4: all runtimes, not claude-only).
func supportFor(t model.ArtifactType, rt model.Runtime) string {
	return string(capability.Level(t, rt))
}

// classifyOutput maps one emitted file path to its CC-3 kind (+ key/sentinel).
// The classification is by the install-merge strategy the file implies:
//   - .mcp.json / settings.json → mergeJSONKey (mcpServers.<name>)
//   - config.toml               → mergeTOMLKey (mcp_servers.<name>)
//   - GEMINI.md / AGENTS.md      → sentinel (<bundle>/<name>)
//   - everything else            → file
func classifyOutput(a *model.Artifact, path string) Output {
	switch {
	case strings.HasSuffix(path, ".mcp.json"), strings.HasSuffix(path, "settings.json"):
		return Output{Path: path, Kind: "mergeJSONKey", Key: "mcpServers." + a.Name}
	case strings.HasSuffix(path, "config.toml"):
		return Output{Path: path, Kind: "mergeTOMLKey", Key: "mcp_servers." + a.Name}
	case strings.HasSuffix(path, "GEMINI.md"), strings.HasSuffix(path, "AGENTS.md"):
		return Output{Path: path, Kind: "sentinel", Sentinel: a.Bundle + "/" + a.Name}
	default:
		return Output{Path: path, Kind: "file"}
	}
}

// runtimeOutputs renders the artifact through each targeted runtime's adapter
// (CC-4) and records the emitted files by CC-3 kind, plus a fidelity note for
// emulated runtimes. Render is deterministic, so this is byte-attributable to the
// dist tree the build orchestrator emits.
func runtimeOutputs(a *model.Artifact) (map[string][]Output, map[string]string, error) {
	outputs := map[string][]Output{}
	fidelity := map[string]string{}
	reg := registry.All()
	for _, rt := range a.Runtimes {
		tgt, ok := reg[rt]
		if !ok {
			continue
		}
		emulated := capability.Level(a.Type, rt) == model.SupportEmulated
		files, _, err := tgt.Render(&model.Bundle{Name: a.Bundle, Artifacts: []*model.Artifact{a}})
		if err != nil {
			// A render error on a runtime the artifact opted into is a real fault;
			// surface it (the build verb only renders claude, so nothing else would).
			return nil, nil, fmt.Errorf("render %s/%s on %s: %w", a.Bundle, a.Name, rt, err)
		}
		for _, f := range files {
			// plugin.json is a bundle-level structural file (emitted once per
			// bundle, not per artifact); it has no artifact attribution, so keep
			// it out of the per-artifact output listing.
			if f.Path == ".claude-plugin/plugin.json" {
				continue
			}
			o := classifyOutput(a, f.Path)
			o.Emulated = emulated
			outputs[string(rt)] = append(outputs[string(rt)], o)
		}
		if emulated {
			fidelity[string(rt)] = "emulated — derived shape; may not auto-activate on this runtime; verify."
		}
	}
	return outputs, fidelity, nil
}

// divergedOnClaude reports whether the artifact uses an annotated full-body
// override for the Claude runtime (author divergence, in scope since slice 2).
func divergedOnClaude(a *model.Artifact) bool {
	for _, rt := range a.Runtimes {
		if rt == model.RuntimeClaude {
			_, f, err := merge.Resolve(a, model.RuntimeClaude)
			return err == nil && f.Diverged
		}
	}
	return false
}

// adapterVersions lists the version of every runtime target that contributes to
// this index (CC-2 generatedBy). Sourced from the registry so a target bump shows up.
func adapterVersions() map[string]string {
	av := map[string]string{}
	for rt, tgt := range registry.All() {
		av[string(rt)] = tgt.Version()
	}
	return av
}

// Build returns the lean index and a map of bundle-name -> CC-3 detail. It errors
// if any targeted runtime's render fails (a real engine fault — validation gates
// unsupported types before this point).
func Build(cat *model.Catalog) (Index, map[string]BundleDetail, error) {
	idx := Index{
		SchemaVersion: SchemaVersion,
		GeneratedBy:   GeneratedBy{AdapterVersions: adapterVersions()},
	}
	details := map[string]BundleDetail{}
	for _, b := range cat.Bundles {
		detailArtifacts := make([]DetailEntry, 0, len(b.Artifacts))
		for _, a := range b.Artifacts {
			support := map[string]string{}
			runtimes := make([]string, 0, len(a.Runtimes))
			for _, rt := range a.Runtimes {
				support[string(rt)] = supportFor(a.Type, rt)
				runtimes = append(runtimes, string(rt))
			}
			idx.Artifacts = append(idx.Artifacts, Entry{
				Name:        a.Name,
				Type:        string(a.Type),
				Bundle:      b.Name,
				Description: a.Description,
				Tags:        a.Tags,
				Category:    a.Category,
				Maturity:    string(a.Maturity),
				Version:     a.Version,
				Runtimes:    runtimes,
				Support:     support,
				Digest:      digest.Source(a),
			})

			// CC-3 / CC-4 detail: per-runtime support, outputs, and fidelity notes.
			requires := make([]Requirement, 0, len(a.Requires))
			for _, r := range a.Requires {
				requires = append(requires, Requirement{Type: string(r.Type), Ref: r.Ref})
			}
			outputs, fidelity, err := runtimeOutputs(a)
			if err != nil {
				return Index{}, nil, err
			}
			detailArtifacts = append(detailArtifacts, DetailEntry{
				Name:          a.Name,
				Type:          string(a.Type),
				Description:   a.Description,
				Version:       a.Version,
				Runtimes:      runtimes,
				Support:       support,
				Tools:         a.Tools, // agent tool grants surfaced in detail (spec §7.4 "surfaced in the index")
				Requires:      requires,
				Diverged:      divergedOnClaude(a),
				Outputs:       outputs,
				FidelityNotes: fidelity,
			})
		}
		details[b.Name] = BundleDetail{
			SchemaVersion: SchemaVersion,
			Bundle: BundleMeta{
				Name:        b.Name,
				Version:     b.Version,
				Description: b.Description,
				Category:    b.Category,
				Tags:        b.Tags,
				Owner:       b.Owner.Name,
				Maturity:    string(b.Maturity),
				Homepage:    b.Homepage,
			},
			Artifacts: detailArtifacts,
		}
	}
	return idx, details, nil
}
