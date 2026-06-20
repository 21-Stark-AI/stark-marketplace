package codex

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

var update = flag.Bool("update", false, "update golden files")

func goldenSkill() *model.Artifact {
	return &model.Artifact{
		Name: "stark-review", Type: model.TypeSkill, Bundle: "stark-review",
		Description: "Single-agent PR review.", Version: "0.7.0",
		Model:    "opus", // maps → gpt-5-codex
		Body:     "Do the review.\n",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
}

func goldenMCP() *model.Artifact {
	return &model.Artifact{
		Name: "bigquery", Type: model.TypeMCP, Bundle: "stark-data",
		Description: "BQ MCP.", Version: "1.2.0",
		Runtimes: []model.Runtime{model.RuntimeCodex},
		MCP: &model.MCPConfig{
			Transport: "stdio", Command: "stark-bq-mcp",
			Args: []string{"--project", "${BQ_PROJECT}"},
			Env:  map[string]model.SecretRef{"BQ_PROJECT": {SecretRef: "bq-project-id"}},
		},
	}
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run -update)", name, err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden mismatch %s:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestGoldenCodexSkill(t *testing.T) {
	files, _, _ := New().Render(bundleWith(goldenSkill()))
	assertGolden(t, "skill.golden", files[0].Content)
}

func TestGoldenCodexMCP(t *testing.T) {
	files, _, _ := New().Render(bundleWith(goldenMCP()))
	assertGolden(t, "mcp.golden", files[0].Content)
}
