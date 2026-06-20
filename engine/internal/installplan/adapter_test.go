package installplan

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/indexio"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestFakeAdapterEmitsFromOutputs(t *testing.T) {
	a := &indexio.ArtifactDetail{
		Name: "gh", Type: model.TypeMCP, Version: "1.0.0",
		Outputs: map[model.Runtime][]indexio.Output{
			model.RuntimeCodex: {{Path: "config.toml", Kind: "mergeTOMLKey", Key: "mcp_servers.gh"}},
		},
		MCP: &model.MCPConfig{Transport: "stdio", Command: "node", Args: []string{"x.js"}},
	}
	fa := NewFakeAdapter(map[string]string{"config.toml#mcp_servers.gh": "command = \"node\"\n"})
	arts, err := fa.Adapt("stark-gh", a, model.RuntimeCodex)
	if err != nil || len(arts) != 1 {
		t.Fatalf("adapt failed: %v %+v", err, arts)
	}
	if arts[0].Kind != "mergeTOMLKey" || arts[0].Payload == "" {
		t.Fatalf("artifact wrong: %+v", arts[0])
	}
}
