// Package merge resolves per-runtime frontmatter and body for an artifact.
// It is pure and order-independent (spec §4.3): scalars replace, maps merge,
// arrays replace wholesale.
package merge

import (
	"fmt"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/fence"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// deepMerge returns base with patch applied. Both inputs are treated as
// read-only; the result is a fresh map. Rules (spec §4.3 step 2):
//   - a key present only in base -> kept
//   - a key present in patch with a scalar/array value -> replaces base
//   - a key present in both as maps -> merged recursively
func deepMerge(base, patch map[string]any) map[string]any {
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = cloneValue(v)
	}
	for k, pv := range patch {
		if bv, ok := out[k]; ok {
			bm, bIsMap := bv.(map[string]any)
			pm, pIsMap := pv.(map[string]any)
			if bIsMap && pIsMap {
				out[k] = deepMerge(bm, pm)
				continue
			}
		}
		out[k] = cloneValue(pv) // scalar or array: wholesale replace
	}
	return out
}

func cloneValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return deepMerge(t, map[string]any{})
	case []any:
		cp := make([]any, len(t))
		for i, e := range t {
			cp[i] = cloneValue(e)
		}
		return cp
	default:
		return v
	}
}

// parseDivergedReason extracts a leading `# diverged: <reason>` annotation from a
// full-body-replacement override (spec §4.3 step 4). It returns the reason (empty
// if absent) and the body with that annotation line removed.
func parseDivergedReason(body string) (reason, stripped string) {
	nl := strings.IndexByte(body, '\n')
	first := body
	rest := ""
	if nl >= 0 {
		first = body[:nl]
		rest = body[nl+1:]
	}
	trimmed := strings.TrimSpace(first)
	const prefix = "# diverged:"
	if strings.HasPrefix(trimmed, prefix) {
		return strings.TrimSpace(trimmed[len(prefix):]), rest
	}
	return "", body
}

// arrayDropsPrefix reports whether the override array fails to be a
// superset-by-prefix of the base array (spec §4.3 step 2 foot-gun warning):
// every base element must appear, in order, as a prefix of the override.
func arrayDropsPrefix(base, override []any) bool {
	if len(override) < len(base) {
		return true
	}
	for i := range base {
		if !valueEqual(base[i], override[i]) {
			return true
		}
	}
	return false
}

func valueEqual(a, b any) bool {
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		return as == bs
	}
	// Non-string elements: `requires` entries are map[string]any (== panics on maps),
	// and numbers may be json.Number on the base side (Raw came through a YAML→JSON
	// round-trip with UseNumber) vs Go int on the yaml-decoded override side (== is
	// type-sensitive). Compare by canonical string form instead — fmt prints map keys
	// in sorted order (Go ≥1.12), so this stays deterministic and never panics.
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// Resolved is the per-runtime output of merging an artifact.
type Resolved struct {
	Frontmatter map[string]any // base frontmatter deep-merged with overrides.<runtime>
	Body        string         // fence-stripped portable body, OR the diverged full body
}

// Findings carry non-fatal signals for the engine to surface (spec §4.3).
type Findings struct {
	ArrayDrops     []string // field names whose override array dropped a base prefix
	Diverged       bool     // full-body replacement was used
	DivergedReason string   // reason from the `# diverged:` annotation
}

// arrayFields are the override arrays most prone to the wholesale-replace foot-gun.
var arrayFields = []string{"tags", "requires", "allowed-tools", "tools"}

// Resolve produces the merged frontmatter + body for artifact a on runtime rt.
// It is pure and order-independent. A full-body override without a `# diverged:`
// reason is a lint error (spec §4.3 step 4).
func Resolve(a *model.Artifact, rt model.Runtime) (Resolved, Findings, error) {
	var f Findings
	base, _ := a.Raw.(map[string]any)
	if base == nil {
		base = map[string]any{}
	}
	ov, hasOverride := a.Overrides[rt]
	fm := base
	if hasOverride && len(ov.Fields) > 0 {
		for _, field := range arrayFields {
			ba, baOK := base[field].([]any)
			oa, oaOK := ov.Fields[field].([]any)
			if baOK && oaOK && arrayDropsPrefix(ba, oa) {
				f.ArrayDrops = append(f.ArrayDrops, field)
			}
		}
		// Nested mcp.args is a spec-§4.3-named foot-gun: an override that drops a
		// prefix element of args silently truncates the launch command. The top-level
		// scan can't see it, so descend into the mcp sub-map explicitly.
		if bm, ok := base["mcp"].(map[string]any); ok {
			if om, ok := ov.Fields["mcp"].(map[string]any); ok {
				ba, baOK := bm["args"].([]any)
				oa, oaOK := om["args"].([]any)
				if baOK && oaOK && arrayDropsPrefix(ba, oa) {
					f.ArrayDrops = append(f.ArrayDrops, "mcp.args")
				}
			}
		}
		fm = deepMerge(base, ov.Fields)
	}

	// Body: full-body replacement (annotated) wins; else fence-strip the base body.
	if hasOverride && ov.Body != "" {
		reason, stripped := parseDivergedReason(ov.Body)
		if reason == "" {
			return Resolved{}, f, fmt.Errorf(
				"%s/%s: full-body override for runtime %q requires a `# diverged: <reason>` annotation (spec §4.3)",
				a.Bundle, a.Name, rt)
		}
		f.Diverged = true
		f.DivergedReason = reason
		return Resolved{Frontmatter: fm, Body: stripped}, f, nil
	}

	body, err := fence.Strip(a.Body, rt, a.Runtimes)
	if err != nil {
		return Resolved{}, f, fmt.Errorf("%s/%s: %w", a.Bundle, a.Name, err)
	}
	return Resolved{Frontmatter: fm, Body: body}, f, nil
}
