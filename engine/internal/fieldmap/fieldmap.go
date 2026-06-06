// Package fieldmap is the table-driven per-field capability fallback (spec §6.2).
// It tells each adapter target how to treat a canonical field on a given runtime:
// carry it natively, translate its value, drop it (counting a warning), derive it
// into prose, or block.
package fieldmap

import "github.com/GetEvinced/stark-marketplace/engine/internal/model"

type Action string

const (
	ActionCarry      Action = "carry"       // emit as a native field
	ActionMap        Action = "map"         // translate the value
	ActionDrop       Action = "drop+warn"   // omit; count a warning
	ActionDerive     Action = "derive"      // render into usage/prose text
	ActionBestEffort Action = "best-effort" // emit as best-effort metadata
	ActionError      Action = "error"       // block the build
)

type key struct {
	field string
	rt    model.Runtime
}

// table is the §6.2 contract for the common canonical fields. Anything not listed
// defaults to carry. `allowed-tools` and `tools` share row 4 of the §6.2 table.
var table = map[key]Action{
	// model
	{"model", model.RuntimeClaude}: ActionCarry, // skill: only with context:fork (target enforces)
	{"model", model.RuntimeCodex}:  ActionMap,
	{"model", model.RuntimeGemini}: ActionDrop,
	// argument-hint
	{"argument-hint", model.RuntimeClaude}: ActionCarry,
	{"argument-hint", model.RuntimeCodex}:  ActionDerive,
	{"argument-hint", model.RuntimeGemini}: ActionDerive,
	// disable-model-invocation
	{"disable-model-invocation", model.RuntimeClaude}: ActionCarry,
	{"disable-model-invocation", model.RuntimeCodex}:  ActionDrop,
	{"disable-model-invocation", model.RuntimeGemini}: ActionDrop,
	// allowed-tools
	{"allowed-tools", model.RuntimeClaude}: ActionCarry,
	{"allowed-tools", model.RuntimeCodex}:  ActionBestEffort,
	{"allowed-tools", model.RuntimeGemini}: ActionDrop,
	// tools (agent tool-allowlist) — same §6.2 row as allowed-tools.
	{"tools", model.RuntimeClaude}: ActionCarry,
	{"tools", model.RuntimeCodex}:  ActionBestEffort,
	{"tools", model.RuntimeGemini}: ActionDrop,
}

func actionFor(field string, rt model.Runtime) Action {
	if a, ok := table[key{field, rt}]; ok {
		return a
	}
	return ActionCarry
}

// ModelMapper translates a canonical model id into a runtime-specific id. ok=false
// means "no mapping" → the field drops with a warning.
type ModelMapper func(canonical string) (string, bool)

// Result is the outcome of applying the field map to one artifact for one runtime.
// Carried values keep their native Go type (string scalars, []string lists) so the
// target can emit lists as YAML sequences rather than comma-joined scalars.
type Result struct {
	Carried map[string]any    // field -> native value to emit
	Derived map[string]string // field -> value to render into usage/prose
	Dropped []string          // fields omitted; the target surfaces these as warnings
}

// Apply resolves the common canonical fields for runtime rt. Values come from the
// override-merged frontmatter `fm` (merge.Resolve output) so per-runtime overrides
// are honored, falling back to the typed artifact fields when `fm` doesn't carry a
// key (e.g. inherited values, or tests with an empty Raw). modelMap is consulted
// only for ActionMap on `model`.
func Apply(fm map[string]any, a *model.Artifact, rt model.Runtime, modelMap ModelMapper) Result {
	res := Result{Carried: map[string]any{}, Derived: map[string]string{}}

	mdl := pickString(fm, "model", a.Model)
	hint := pickString(fm, "argument-hint", a.ArgumentHint)
	dmi, dmiPresent := pickBool(fm, "disable-model-invocation", a.DisableModelInvocation)
	allowed := pickStrings(fm, "allowed-tools", a.AllowedTools)
	tools := pickStrings(fm, "tools", a.Tools)

	type field struct {
		name    string
		present bool
		value   any
	}
	fields := []field{
		{"model", mdl != "", mdl},
		{"argument-hint", hint != "", hint},
		{"disable-model-invocation", dmiPresent, dmi},
		{"allowed-tools", len(allowed) > 0, allowed},
		{"tools", len(tools) > 0, tools},
	}

	for _, f := range fields {
		if !f.present {
			continue
		}
		switch actionFor(f.name, rt) {
		case ActionCarry, ActionBestEffort:
			res.Carried[f.name] = f.value
		case ActionDerive:
			if s, ok := f.value.(string); ok {
				res.Derived[f.name] = s
			}
		case ActionMap:
			if f.name == "model" && modelMap != nil {
				if s, ok := f.value.(string); ok {
					if mapped, ok := modelMap(s); ok {
						res.Carried[f.name] = mapped
						continue
					}
				}
			}
			res.Dropped = append(res.Dropped, f.name)
		case ActionDrop, ActionError:
			res.Dropped = append(res.Dropped, f.name)
		}
	}
	return res
}

// pickString returns fm[key] as a string when present, else the typed fallback.
func pickString(fm map[string]any, key, fallback string) string {
	if v, ok := fm[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return fallback
}

// pickBool returns (value, present). A key explicitly present in fm wins (even
// false); otherwise a true typed field counts as present, a false one as absent.
func pickBool(fm map[string]any, key string, typed bool) (bool, bool) {
	if v, ok := fm[key]; ok {
		if b, ok := v.(bool); ok {
			return b, true
		}
	}
	if typed {
		return true, true
	}
	return false, false
}

// pickStrings returns fm[key] as []string (from a JSON/YAML []any) when present,
// else the typed fallback slice.
func pickStrings(fm map[string]any, key string, fallback []string) []string {
	if v, ok := fm[key]; ok {
		if arr, ok := v.([]any); ok {
			out := make([]string, 0, len(arr))
			for _, e := range arr {
				if s, ok := e.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return fallback
}
