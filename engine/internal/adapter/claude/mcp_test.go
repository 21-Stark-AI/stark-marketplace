package claude

import (
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestRenderMCPJSON(t *testing.T) {
	b := &model.Bundle{Name: "demo", Runtimes: []model.Runtime{model.RuntimeClaude}, Artifacts: []*model.Artifact{
		{Name: "zeta", Type: model.TypeMCP, Version: "1.0.0", Runtimes: []model.Runtime{model.RuntimeClaude},
			MCP: &model.MCPConfig{Transport: "stdio", Command: "node", Args: []string{"z.js"},
				Env: map[string]model.SecretRef{"TOKEN": {SecretRef: "gh-token"}}}},
		{Name: "alpha", Type: model.TypeMCP, Version: "1.0.0", Runtimes: []model.Runtime{model.RuntimeClaude},
			MCP: &model.MCPConfig{Transport: "stdio", Command: "uvx", Args: []string{"a"}}},
	}}
	files, _, err := New().Render(b)
	if err != nil {
		t.Fatal(err)
	}
	var mcp string
	for _, f := range files {
		if f.Path == ".mcp.json" {
			mcp = string(f.Content)
		}
	}
	if mcp == "" {
		t.Fatal("no .mcp.json emitted")
	}
	// servers sorted -> alpha appears before zeta
	if strings.Index(mcp, "\"alpha\"") > strings.Index(mcp, "\"zeta\"") {
		t.Fatalf("servers not sorted: %s", mcp)
	}
	if !strings.Contains(mcp, "\"secretRef\": \"gh-token\"") {
		t.Fatalf("secretRef not preserved: %s", mcp)
	}
}
