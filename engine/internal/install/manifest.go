package install

import (
	"encoding/json"
	"os"
	"sort"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// Action records how a path was mutated so --remove can excise precisely (spec §9.2).
type Action string

const (
	ActionWriteFile     Action = "writeFile"     // whole file owned by stark — remove deletes it
	ActionMergeJSONKey  Action = "mergeJSONKey"  // delete only Key from a JSON object
	ActionMergeTOMLKey  Action = "mergeTOMLKey"  // delete only Key table from config.toml
	ActionSentinelBlock Action = "sentinelBlock" // delete only the sentinel-wrapped region
)

// Record is one mutation stark performed.
type Record struct {
	Bundle   string             `json:"bundle"`
	Name     string             `json:"name"`
	Type     model.ArtifactType `json:"type"`
	Path     string             `json:"path"` // relative to dest root
	Action   Action             `json:"action"`
	Key      string             `json:"key,omitempty"`      // for merge actions
	Sentinel string             `json:"sentinel,omitempty"` // for sentinel action
	Digest   string             `json:"digest"`
}

// Manifest is the per-(runtime, dest-root) installed-state record.
type Manifest struct {
	SchemaVersion int           `json:"schemaVersion"`
	Runtime       model.Runtime `json:"runtime"`
	Records       []Record      `json:"records"`
}

func (m *Manifest) Add(r Record) { m.Records = append(m.Records, r) }

// sortRecords yields deterministic on-disk order.
func (m *Manifest) sortRecords() {
	sort.Slice(m.Records, func(i, j int) bool {
		a, b := m.Records[i], m.Records[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Key < b.Key
	})
}

func SaveManifest(path string, m *Manifest) error {
	m.sortRecords()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
