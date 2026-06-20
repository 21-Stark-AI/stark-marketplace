package indexio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// Entry is one lean search row (spec §7.5, CC-2). Additive-only within a schemaVersion;
// unknown fields are ignored by the json decoder by construction. The lean index holds
// ONLY artifact rows — bundle-level metadata lives in bundles/<name>.json (no "bundle"
// type rows here).
type Entry struct {
	Name        string                               `json:"name"`
	Type        model.ArtifactType                   `json:"type"`
	Bundle      string                               `json:"bundle"`
	Description string                               `json:"description,omitempty"`
	Tags        []string                             `json:"tags,omitempty"`
	Category    string                               `json:"category,omitempty"`
	Maturity    model.Maturity                       `json:"maturity,omitempty"`
	Version     string                               `json:"version"`
	Support     map[model.Runtime]model.SupportLevel `json:"support,omitempty"`
	Digest      string                               `json:"digest,omitempty"`
}

// Index is the lean committed index.json. Top-level key is "artifacts" (CC-2).
type Index struct {
	SchemaVersion int     `json:"schemaVersion"`
	Artifacts     []Entry `json:"artifacts"`
}

// Find returns the entry for (bundle, name, type) or nil.
func (i *Index) Find(bundle, name string, typ model.ArtifactType) *Entry {
	for idx := range i.Artifacts {
		e := &i.Artifacts[idx]
		if e.Bundle == bundle && e.Name == name && e.Type == typ {
			return e
		}
	}
	return nil
}

// Output describes one file a runtime adapter emits for an artifact (CC-3).
// Kind ∈ {file, mergeJSONKey, mergeTOMLKey, sentinel}. Key is a dotted path for the
// merge kinds (e.g. "mcp_servers.gh"); Sentinel is the bundle/name id for sentinel kind.
type Output struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Key      string `json:"key,omitempty"`
	Sentinel string `json:"sentinel,omitempty"`
	Emulated bool   `json:"emulated,omitempty"`
}

// ArtifactDetail is the per-bundle detail row (spec §7.5, CC-3).
type ArtifactDetail struct {
	Name          string                               `json:"name"`
	Type          model.ArtifactType                   `json:"type"`
	Description   string                               `json:"description,omitempty"`
	Version       string                               `json:"version"`
	Runtimes      []model.Runtime                      `json:"runtimes"`
	Support       map[model.Runtime]model.SupportLevel `json:"support"`
	Requires      []model.Requirement                  `json:"requires,omitempty"`
	Diverged      bool                                 `json:"diverged"`
	Outputs       map[model.Runtime][]Output           `json:"outputs"`
	MCP           *model.MCPConfig                     `json:"mcp,omitempty"`
	FidelityNotes map[model.Runtime]string             `json:"fidelityNotes,omitempty"`
}

// BundleMeta is the bundle-level metadata block (CC-3 "bundle" object). This is the only
// place bundle-level fields live — the lean index carries no bundle rows.
type BundleMeta struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description,omitempty"`
	Category    string         `json:"category,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Owner       string         `json:"owner,omitempty"`
	Maturity    model.Maturity `json:"maturity,omitempty"`
	Homepage    string         `json:"homepage,omitempty"`
}

// BundleDetail is bundles/<name>.json (CC-3).
type BundleDetail struct {
	SchemaVersion int              `json:"schemaVersion"`
	Bundle        BundleMeta       `json:"bundle"`
	Artifacts     []ArtifactDetail `json:"artifacts"`
}

// Artifact returns the detail row for (name, type) or nil.
func (d *BundleDetail) Artifact(name string, typ model.ArtifactType) *ArtifactDetail {
	for idx := range d.Artifacts {
		a := &d.Artifacts[idx]
		if a.Name == name && a.Type == typ {
			return a
		}
	}
	return nil
}

// LoadIndex reads + asserts the schemaVersion of index.json.
func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}
	if err := AssertSchemaVersion(idx.SchemaVersion); err != nil {
		return nil, err
	}
	return &idx, nil
}

// LoadBundleDetail reads bundles/<name>.json and asserts its schemaVersion.
func LoadBundleDetail(bundlesDir, name string) (*BundleDetail, error) {
	data, err := os.ReadFile(filepath.Join(bundlesDir, name+".json"))
	if err != nil {
		return nil, err
	}
	var d BundleDetail
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse bundle detail %s: %w", name, err)
	}
	if err := AssertSchemaVersion(d.SchemaVersion); err != nil {
		return nil, err
	}
	return &d, nil
}
