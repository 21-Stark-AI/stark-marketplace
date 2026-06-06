package installplan

import (
	"strings"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/indexio"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestAddConsentMCPAndAgent(t *testing.T) {
	var cp ConsentPayload
	addConsent(&cp, node{"b", "bq", model.TypeMCP}, &indexio.ArtifactDetail{
		Name: "bq", Type: model.TypeMCP,
		MCP: &model.MCPConfig{Transport: "stdio", Command: "node", Args: []string{"bq.js", "--x"}},
	})
	addConsent(&cp, node{"b", "rt", model.TypeAgent}, &indexio.ArtifactDetail{
		Name: "rt", Type: model.TypeAgent,
	})
	if !cp.Required {
		t.Fatal("mcp/agent must require consent")
	}
	if len(cp.MCPCommands) != 1 || !strings.Contains(cp.MCPCommands[0], "node bq.js --x") {
		t.Fatalf("mcp command line wrong: %+v", cp.MCPCommands)
	}
	if len(cp.ClosureRefs) != 2 {
		t.Fatalf("closure refs = %+v", cp.ClosureRefs)
	}
}
