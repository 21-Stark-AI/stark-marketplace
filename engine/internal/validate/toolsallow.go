package validate

import "github.com/GetEvinced/stark-marketplace/engine/internal/model"

// agentToolAllowlist is the known-safe set of agent tool grants (spec §7.4 "agent.tools
// validated against an allowlist and surfaced in the index"). This file owns the VALIDATION
// half — an unknown tool is a WARNING (visible in PR output), not a hard error; new tools are
// added here through the governance process in docs/SECURITY.md. The SURFACING half lives in
// the CC-3 bundle detail: index.DetailEntry.Tools carries the grant list for the install-consent UX.
var agentToolAllowlist = map[string]bool{
	"Bash": true, "Read": true, "Edit": true, "Write": true, "Grep": true,
	"Glob": true, "WebFetch": true, "WebSearch": true, "Task": true,
	"NotebookEdit": true, "TodoWrite": true,
}

func checkAgentTools(r *Result, where string, a *model.Artifact) {
	if a.Type != model.TypeAgent {
		return
	}
	for _, tool := range a.Tools {
		if !agentToolAllowlist[tool] {
			r.Warnf(where, "agent.tools grants unknown tool %q (not on allowlist; surfaced for review)", tool)
		}
	}
}
