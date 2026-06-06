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
// defaults to carry.
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
	// allowed-tools / tools
	{"allowed-tools", model.RuntimeClaude}: ActionCarry,
	{"allowed-tools", model.RuntimeCodex}:  ActionBestEffort,
	{"allowed-tools", model.RuntimeGemini}: ActionDrop,
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
type Result struct {
	Carried map[string]string // field -> native value to emit
	Derived map[string]string // field -> value to render into usage/prose
	Dropped []string          // fields omitted; caller counts these as warnings
}

// Apply walks the common canonical fields present on a, resolves each via the
// §6.2 table, and partitions them into carried / derived / dropped. modelMap is
// consulted only for ActionMap on the `model` field.
func Apply(a *model.Artifact, rt model.Runtime, modelMap ModelMapper) Result {
	res := Result{Carried: map[string]string{}, Derived: map[string]string{}}

	type field struct {
		name    string
		present bool
		value   string
	}
	fields := []field{
		{"model", a.Model != "", a.Model},
		{"argument-hint", a.ArgumentHint != "", a.ArgumentHint},
		{"disable-model-invocation", a.DisableModelInvocation, boolStr(a.DisableModelInvocation)},
		{"allowed-tools", len(a.AllowedTools) > 0, joinTools(a.AllowedTools)},
	}

	for _, f := range fields {
		if !f.present {
			continue
		}
		switch actionFor(f.name, rt) {
		case ActionCarry, ActionBestEffort:
			res.Carried[f.name] = f.value
		case ActionDerive:
			res.Derived[f.name] = f.value
		case ActionMap:
			if f.name == "model" && modelMap != nil {
				if mapped, ok := modelMap(f.value); ok {
					res.Carried[f.name] = mapped
					continue
				}
			}
			res.Dropped = append(res.Dropped, f.name)
		case ActionDrop, ActionError:
			res.Dropped = append(res.Dropped, f.name)
		}
	}
	return res
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func joinTools(t []string) string {
	out := ""
	for i, s := range t {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}
