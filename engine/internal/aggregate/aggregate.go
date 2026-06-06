// Package aggregate merges N per-artifact contributions destined for ONE shared
// file (GEMINI.md, AGENTS.md, a single config.toml) into a deterministic,
// sentinel-wrapped document (spec §6.3). Sections are sorted by <bundle>/<name>
// and the merge is idempotent on rebuild. Install merges by sentinel, never
// blind-append (§9.2).
package aggregate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Section is one artifact's contribution to a shared file.
type Section struct {
	Bundle  string
	Name    string
	Content string // inner content (already includes any fidelity header)
}

func (s Section) id() string { return s.Bundle + "/" + s.Name }

func digest(inner string) string {
	sum := sha256.Sum256([]byte(inner))
	return hex.EncodeToString(sum[:])[:12]
}

// Merge wraps each section in stable sentinels, sorts by <bundle>/<name>, and
// concatenates. Pure + deterministic; running it on its own output is a no-op
// modulo re-parse (see property test).
func Merge(sections []Section) string {
	if len(sections) == 0 {
		return ""
	}
	sorted := make([]Section, len(sections))
	copy(sorted, sections)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].id() < sorted[j].id() })

	var b strings.Builder
	for _, s := range sorted {
		inner := s.Content
		if !strings.HasSuffix(inner, "\n") {
			inner += "\n"
		}
		fmt.Fprintf(&b, "<!-- stark:begin %s@%s -->\n", s.id(), digest(inner))
		b.WriteString(inner)
		fmt.Fprintf(&b, "<!-- stark:end %s -->\n", s.id())
	}
	return b.String()
}

var (
	beginRe = regexp.MustCompile(`^<!--\s*stark:begin\s+(\S+?)/(\S+?)@[0-9a-f]+\s*-->$`)
	endRe   = regexp.MustCompile(`^<!--\s*stark:end\s+(\S+?)/(\S+?)\s*-->$`)
)

// Parse extracts the managed sections from a previously-merged document. Content
// outside sentinels is ignored (install preserves it separately). The digest in
// the begin sentinel is dropped — Merge recomputes it — so Parse→Merge is stable.
func Parse(doc string) []Section {
	lines := strings.Split(doc, "\n")
	var out []Section
	var cur *Section
	var buf []string
	for _, ln := range lines {
		if m := beginRe.FindStringSubmatch(ln); m != nil {
			cur = &Section{Bundle: m[1], Name: m[2]}
			buf = nil
			continue
		}
		if m := endRe.FindStringSubmatch(ln); m != nil && cur != nil {
			cur.Content = strings.Join(buf, "\n")
			if cur.Content != "" {
				cur.Content += "\n"
			}
			out = append(out, *cur)
			cur = nil
			continue
		}
		if cur != nil {
			buf = append(buf, ln)
		}
	}
	return out
}
