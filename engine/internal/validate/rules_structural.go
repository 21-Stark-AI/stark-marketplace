package validate

import (
	"regexp"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func checkSlug(r *Result, where, s string) {
	if !slugRe.MatchString(s) {
		r.Errorf(where, "invalid slug %q (want ^[a-z0-9][a-z0-9-]{0,63}$)", s)
	}
}

func checkRuntimesNarrowing(r *Result, where string, a *model.Artifact, b *model.Bundle) {
	if len(b.Runtimes) == 0 || len(a.Runtimes) == 0 {
		return
	}
	allowed := map[model.Runtime]bool{}
	for _, rt := range b.Runtimes {
		allowed[rt] = true
	}
	for _, rt := range a.Runtimes {
		if !allowed[rt] {
			r.Errorf(where, "runtime %q widens beyond bundle's set", rt)
		}
	}
}

// outputNamespace returns a per-runtime logical namespace key used to detect
// cross-type file collisions (spec §5.2). This is a conservative model of the
// adapter's path mapping; plan 03 keeps it in sync with real target paths.
func outputNamespace(t model.ArtifactType, rt model.Runtime) string {
	switch rt {
	case model.RuntimeCodex:
		// On Codex, skill/prompt/command AND agent all emit
		// .agents/skills/<name>/SKILL.md (agent is emulated as a skill), so they
		// share one output namespace — a skill "x" and agent "x" would overwrite
		// each other and MUST be reported as a collision (CC-7 / plan 03 Task 17).
		switch t {
		case model.TypeSkill, model.TypePrompt, model.TypeCommand, model.TypeAgent:
			return "codex:skilllike"
		default:
			return "codex:" + string(t)
		}
	case model.RuntimeGemini:
		switch t {
		case model.TypePrompt, model.TypeCommand:
			return "gemini:command"
		default:
			return "gemini:" + string(t)
		}
	default: // claude keeps types separate
		return "claude:" + string(t)
	}
}

func checkOutputUniqueness(r *Result, b *model.Bundle) {
	seen := map[string]string{} // namespace+name -> first owner
	for _, a := range b.Artifacts {
		for _, rt := range a.Runtimes {
			key := outputNamespace(a.Type, rt) + "/" + a.Name
			if prev, ok := seen[key]; ok {
				r.Errorf(b.Name, "output collision on %q: %s and %s/%s map to the same %s file",
					a.Name, prev, b.Name, a.Type, rt)
			} else {
				seen[key] = b.Name + "/" + string(a.Type)
			}
		}
	}
}
