package installplan

import (
	"fmt"
	"strings"

	"github.com/21StarkCom/bifrost/engine/internal/indexio"
	"github.com/21StarkCom/bifrost/engine/internal/model"
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

// addAssetConsent flags a bundle's vendored asset step for consent when it carries
// executable code (tools/*.ts, scripts/*.sh). Those files are not an artifact class,
// but the installed skills shell out to them, so installing them is a code-execution
// surface that belongs inside the §9.3 gate — otherwise a skill-only bundle writes
// vendor-supplied executables with no confirmation at all.
func addAssetConsent(cp *ConsentPayload, bundle string, files []AdaptedFile) {
	n := 0
	for _, f := range files {
		if isExecutableAsset(f.Path) {
			n++
		}
	}
	if n == 0 {
		return
	}
	cp.Required = true
	cp.AssetExec = append(cp.AssetExec, fmt.Sprintf("%s: %d executable vendored files (tools/scripts)", bundle, n))
}

// isExecutableAsset reports whether a vendored asset path is code the installed skills
// execute — the TypeScript tool scripts and shell scripts. Everything else (config.json,
// standards/prompts markdown) is inert data the consent gate need not cover.
func isExecutableAsset(path string) bool {
	return strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".sh") ||
		strings.HasSuffix(path, ".mjs") || strings.HasSuffix(path, ".cjs") ||
		strings.HasSuffix(path, ".js")
}
