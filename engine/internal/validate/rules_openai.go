package validate

import "github.com/21StarkCom/bifrost/engine/internal/model"

// checkOpenAICompatibility surfaces the marketplace policy that Claude-facing
// skills and slash commands should also be installable on the OpenAI/Codex
// surface. A claude-without-codex artifact can only reach validation through an
// explicit `runtimes: [claude]` declaration in source frontmatter (bundle
// inheritance and the importer default both include codex), so the gap is a
// deliberate authorial narrowing — e.g. a skill whose verbs are Claude-session
// protocol — and reports as a warning, not an error.
func checkOpenAICompatibility(r *Result, where string, a *model.Artifact) {
	if a.Type != model.TypeSkill && a.Type != model.TypeCommand {
		return
	}
	if hasRuntime(a.Runtimes, model.RuntimeClaude) && !hasRuntime(a.Runtimes, model.RuntimeCodex) {
		r.Warnf(where, "Claude-only %s ships without a codex variant (deliberate runtimes narrowing)", a.Type)
	}
}

func hasRuntime(runtimes []model.Runtime, want model.Runtime) bool {
	for _, rt := range runtimes {
		if rt == want {
			return true
		}
	}
	return false
}
