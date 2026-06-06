package importer

import (
	"strings"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestSerializeArtifactMarkdown(t *testing.T) {
	a := &model.Artifact{
		Name: "demo-review", Type: model.TypeSkill, Description: "PR review.",
		Version: "0.1.0", Maturity: model.MaturityBeta,
		Runtimes:     []model.Runtime{model.RuntimeClaude},
		ArgumentHint: "[PR]", Model: "opus[1m]",
		Body: "Body line one.\n",
	}
	out, err := serializeArtifact(a)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// frontmatter fenced
	if !strings.HasPrefix(s, "---\n") || strings.Count(s, "---\n") < 2 {
		t.Fatalf("missing frontmatter fences:\n%s", s)
	}
	// identity fields present and ordered: name before type before description
	iName := strings.Index(s, "name:")
	iType := strings.Index(s, "type:")
	iDesc := strings.Index(s, "description:")
	if !(iName < iType && iType < iDesc) {
		t.Fatalf("frontmatter not in canonical order:\n%s", s)
	}
	// body preserved verbatim after closing fence
	if !strings.HasSuffix(s, "Body line one.\n") {
		t.Fatalf("body not preserved:\n%s", s)
	}
}

func TestSerializeMCPYAML(t *testing.T) {
	a := &model.Artifact{
		Name: "gh", Type: model.TypeMCP, Description: "GitHub MCP.",
		Version: "0.1.0", Runtimes: []model.Runtime{model.RuntimeClaude},
		MCP: &model.MCPConfig{
			Transport: "stdio", Command: "node", Args: []string{"gh-mcp-server.js"},
			Env: map[string]model.SecretRef{"GITHUB_TOKEN": {SecretRef: "stark-gh-token"}},
		},
	}
	out, err := serializeArtifact(a)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.HasPrefix(s, "---\n") {
		t.Fatalf("mcp must be plain YAML, not frontmatter:\n%s", s)
	}
	if !strings.Contains(s, "secretRef: stark-gh-token") {
		t.Fatalf("secretRef not serialized:\n%s", s)
	}
}
