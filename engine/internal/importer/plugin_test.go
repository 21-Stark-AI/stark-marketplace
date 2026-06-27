package importer

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestImportPluginCommandsAndMCP(t *testing.T) {
	res, err := Import(Options{From: "testdata/stark-skills", Bundle: "stark-gh"})
	if err != nil {
		t.Fatal(err)
	}
	pr := findArtifact(res.Bundle, "pr-open")
	if pr == nil || pr.Type != model.TypeCommand {
		t.Fatalf("pr-open command not imported: %+v", pr)
	}
	if pr.ArgumentHint == "" || len(pr.AllowedTools) == 0 {
		t.Fatalf("command fields not carried: %+v", pr)
	}
	gh := findArtifact(res.Bundle, "gh")
	if gh == nil || gh.Type != model.TypeMCP || gh.MCP == nil {
		t.Fatalf("gh mcp not imported: %+v", gh)
	}
	if gh.MCP.Command != "node" || gh.MCP.Env["GITHUB_TOKEN"].SecretRef != "stark-gh-token" {
		t.Fatalf("mcp payload wrong: %+v", gh.MCP)
	}
	// bundle metadata seeded from plugin.json
	if res.Bundle.Owner.Name != "21 Stark AI" || res.Bundle.Description == "" {
		t.Fatalf("bundle meta from plugin.json missing: %+v", res.Bundle)
	}
}
