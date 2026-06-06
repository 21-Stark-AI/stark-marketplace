// Package provenance builds and verifies the CI-signed build manifest (spec §7.5): a record of
// adapter target versions + content digests for one build. The cosign keyless signature — not
// these self-computed digests — is the provenance root; the digests are an anti-drift layer.
package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// SchemaVersion of the build manifest format. Bump only on breaking changes.
const SchemaVersion = 1

// FileDigest binds a committed generated path to its sha256.
type FileDigest struct {
	Path   string `json:"path"`
	Digest string `json:"digest"` // sha256 hex
}

// BuildManifest is the CI-signed record of (adapter target versions + content digests) for one
// build (spec §7.5). It is signed via cosign keyless; the signature — not these self-computed
// digests — is the provenance root.
type BuildManifest struct {
	SchemaVersion  int            `json:"schemaVersion"`
	TargetVersions map[string]int `json:"targetVersions"` // runtime -> adapter target version
	Files          []FileDigest   `json:"files"`          // sorted by path
}

// Compute builds a deterministic manifest from adapter target versions and the generated file
// bytes. Output is a pure function of inputs: target map is emitted sorted by key (via
// encoding/json map-key sort), files sorted by path.
func Compute(targetVersions map[string]int, files map[string][]byte) *BuildManifest {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	fds := make([]FileDigest, 0, len(paths))
	for _, p := range paths {
		sum := sha256.Sum256(files[p])
		fds = append(fds, FileDigest{Path: p, Digest: hex.EncodeToString(sum[:])})
	}

	tv := make(map[string]int, len(targetVersions))
	for k, v := range targetVersions {
		tv[k] = v
	}
	return &BuildManifest{
		SchemaVersion:  SchemaVersion,
		TargetVersions: tv,
		Files:          fds,
	}
}

// Marshal renders the manifest as indented JSON. encoding/json sorts map keys, and Files is
// pre-sorted, so the output is byte-stable for identical inputs.
func (m *BuildManifest) Marshal() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}
