package installplan

import (
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/indexio"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
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
		grants := "(none)"
		// agent tool grants live on the detail via outputs/metadata; surface raw if present.
		cp.AgentToolGrants = append(cp.AgentToolGrants, a.Name+": "+grants)
	}
}
