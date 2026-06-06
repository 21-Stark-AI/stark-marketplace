// Package bumps implements the version-bump immutability gate (spec §11, CC-5):
// when an artifact's canonical-source digest changes, its version MUST be bumped.
// It compares a previous committed index against the current catalog's recomputed
// source digests. An empty/nil previous map means "first commit" and skips.
package bumps

// Previous is one artifact row from the previously committed index.json.
type Previous struct {
	Version string
	Digest  string // the lean index "digest" field == digest.Source at commit time
}

// Current is one artifact's freshly recomputed identity from the working catalog.
type Current struct {
	Version      string
	SourceDigest string // digest.Source(a) recomputed now
}

// Violation names an artifact whose source changed without a version bump.
type Violation struct {
	Key       string // "<bundle>/<name>"
	Version   string // the unchanged version
	OldDigest string
	NewDigest string
}

// Check returns the immutability violations. A nil/empty previous map skips the
// gate (first commit). Artifacts present only in cur (new) never violate.
func Check(prev map[string]Previous, cur map[string]Current) []Violation {
	if len(prev) == 0 {
		return nil
	}
	var out []Violation
	for key, c := range cur {
		p, ok := prev[key]
		if !ok {
			continue // brand-new artifact
		}
		if c.SourceDigest != p.Digest && c.Version == p.Version {
			out = append(out, Violation{
				Key: key, Version: c.Version, OldDigest: p.Digest, NewDigest: c.SourceDigest,
			})
		}
	}
	return out
}
