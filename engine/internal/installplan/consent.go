package installplan

import (
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/indexio"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// addConsent records consent-relevant facts for an artifact (spec §9.3). Every node lands
// in ClosureRefs; mcp/agent additionally flag Required and list the exact command/grants.
func addConsent(cp *ConsentPayload, n node, a *indexio.ArtifactDetail) {
	tag := n.ref()
	if a.Type == model.TypeMCP || a.Type == model.TypeAgent {
		tag += " [" + string(a.Type) + "]" // highlight transitive code-executing classes
	}
	cp.ClosureRefs = append(cp.ClosureRefs, tag)

	switch a.Type {
	case model.TypeMCP:
		cp.Required = true
		if a.MCP != nil {
			line := a.Name + ": " + a.MCP.Command
			if len(a.MCP.Args) > 0 {
				line += " " + strings.Join(a.MCP.Args, " ")
			}
			cp.MCPCommands = append(cp.MCPCommands, line)
		}
	case model.TypeAgent:
		cp.Required = true
		// The published CC-3 detail does not carry an agent's tool grants, so we cannot
		// enumerate them here — be explicit rather than implying "(none)" granted. The safety
		// gate (Required=true) still fires; a reviewer must inspect the agent before consenting.
		// (Surfacing exact grants needs a CC-3 contract extension — tracked, out of slice 05.)
		cp.AgentToolGrants = append(cp.AgentToolGrants, a.Name+": tool grants not published in index — review the agent before granting")
	}
}
