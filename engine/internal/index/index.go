// Package index projects a loaded catalog into the lean search index
// (index.json) plus per-bundle detail files (bundles/<name>.json), per spec §7.5.
// The lean index carries only what search needs; consumers ignore unknown fields.
package index

import (
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/adapter"
	"github.com/GetEvinced/stark-marketplace/engine/internal/adapter/claude"
	"github.com/GetEvinced/stark-marketplace/engine/internal/digest"
	"github.com/GetEvinced/stark-marketplace/engine/internal/merge"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
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
	Support       map[string]string   `json:"support"`       // runtime -> native|emulated|unsupported
	Requires      []Requirement       `json:"requires"`      // [] not null
	Diverged      bool                `json:"diverged"`
	Outputs       map[string][]Output `json:"outputs"`       // runtime -> output files (claude only in slice 2)
	FidelityNotes map[string]string   `json:"fidelityNotes"` // runtime -> note (claude only in slice 2)
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

// supportFor returns the capability badge for (type, runtime). Slice 2 implements
// Claude only; Codex/Gemini badges are filled by slice 3's capability matrix.
func supportFor(t model.ArtifactType, rt model.Runtime) string {
	if rt == model.RuntimeClaude {
		return string(model.SupportNative) // all five types are native on Claude (spec §6)
	}
	return "" // unknown until slice 3 wires the matrix
}

// claudeOutputsByArtifact renders the bundle with the Claude target and groups
// the resulting output paths by artifact name. The Claude path convention
// (skills/<name>/SKILL.md, commands/<name>.md, agents/<name>.md, .mcp.json) lets
// us attribute each file back to its artifact deterministically.
func claudeOutputsByArtifact(b *model.Bundle) map[string][]Output {
	files, _, err := claude.New().Render(b)
	if err != nil {
		return map[string][]Output{} // render errors surface in the build orchestrator
	}
	byArtifact := map[string][]Output{}
	for _, a := range b.Artifacts {
		for _, f := range claudeFilesForArtifact(a, files) {
			byArtifact[a.Name] = append(byArtifact[a.Name], f)
		}
	}
	return byArtifact
}

// claudeFilesForArtifact picks the OutputFiles that belong to artifact a, mapping
// each to a CC-3 Output. MCP artifacts contribute a mergeJSONKey row keyed by name.
func claudeFilesForArtifact(a *model.Artifact, files []adapter.OutputFile) []Output {
	var out []Output
	switch a.Type {
	case model.TypeMCP:
		for _, f := range files {
			if f.Path == ".mcp.json" {
				out = append(out, Output{Path: ".mcp.json", Kind: "mergeJSONKey", Key: "mcpServers." + a.Name})
			}
		}
	default:
		var prefix string
		switch a.Type {
		case model.TypeSkill:
			prefix = "skills/" + a.Name + "/"
		case model.TypeCommand, model.TypePrompt:
			prefix = "commands/" + a.Name + "."
		case model.TypeAgent:
			prefix = "agents/" + a.Name + "."
		}
		for _, f := range files {
			if prefix != "" && strings.HasPrefix(f.Path, prefix) {
				out = append(out, Output{Path: f.Path, Kind: "file"})
			}
		}
	}
	return out
}

// divergedOnClaude reports whether the artifact uses an annotated full-body
// override for the Claude runtime (author divergence, in scope this slice). It
// only resolves when the artifact actually targets Claude.
func divergedOnClaude(a *model.Artifact) bool {
	for _, rt := range a.Runtimes {
		if rt == model.RuntimeClaude {
			_, f, err := merge.Resolve(a, model.RuntimeClaude)
			return err == nil && f.Diverged
		}
	}
	return false
}

// Build returns the lean index and a map of bundle-name -> CC-3 detail.
func Build(cat *model.Catalog) (Index, map[string]BundleDetail) {
	idx := Index{
		SchemaVersion: SchemaVersion,
		GeneratedBy:   GeneratedBy{AdapterVersions: map[string]string{"claude": claude.Version}},
	}
	details := map[string]BundleDetail{}
	for _, b := range cat.Bundles {
		claudeOut := claudeOutputsByArtifact(b)
		detailArtifacts := make([]DetailEntry, 0, len(b.Artifacts))
		for _, a := range b.Artifacts {
			support := map[string]string{}
			for _, rt := range a.Runtimes {
				if s := supportFor(a.Type, rt); s != "" {
					support[string(rt)] = s
				}
			}
			runtimes := make([]string, 0, len(a.Runtimes))
			for _, rt := range a.Runtimes {
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

			// CC-3 detail. Claude fields are populated from the rendered output;
			// codex/gemini entries are intentionally absent until plan 03 wires the
			// capability matrix + Codex/Gemini targets.
			requires := make([]Requirement, 0, len(a.Requires))
			for _, r := range a.Requires {
				requires = append(requires, Requirement{Type: string(r.Type), Ref: r.Ref})
			}
			outputs := map[string][]Output{}
			fidelity := map[string]string{}
			if outs, ok := claudeOut[a.Name]; ok {
				outputs["claude"] = outs
				fidelity["claude"] = "native"
			}
			// plan 03 fills outputs["codex"]/outputs["gemini"] + their fidelityNotes.
			detailArtifacts = append(detailArtifacts, DetailEntry{
				Name:        a.Name,
				Type:        string(a.Type),
				Description: a.Description,
				Version:     a.Version,
				Runtimes:    runtimes,
				Support:     support,
				Requires:    requires,
				// claude author-divergence (full-body `# diverged:` override) is in
				// scope this slice; plan 03 adds per-runtime divergence for codex/gemini.
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
	return idx, details
}
