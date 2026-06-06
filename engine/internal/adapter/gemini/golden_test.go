package gemini

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

var update = flag.Bool("update", false, "update golden files")

func TestGeminiEmitsMCPSettingsJSON(t *testing.T) {
	a := &model.Artifact{
		Name: "bigquery", Type: model.TypeMCP, Bundle: "stark-data",
		Description: "BQ MCP.", Version: "1.2.0",
		Runtimes: []model.Runtime{model.RuntimeGemini},
		MCP: &model.MCPConfig{
			Transport: "stdio", Command: "stark-bq-mcp",
			Args: []string{"--project", "${BQ_PROJECT}"},
			Env:  map[string]model.SecretRef{"BQ_PROJECT": {SecretRef: "bq-project-id"}},
		},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := find(files, "settings.json")
	if !ok {
		t.Fatalf("expected settings.json; got %v", files)
	}
	for _, want := range []string{`"mcpServers"`, `"bigquery"`, `"command": "stark-bq-mcp"`, `"${BQ_PROJECT}"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("settings.json missing %q in:\n%s", want, body)
		}
	}
}

func TestGeminiMCPEnvMultiKeySorted(t *testing.T) {
	a := &model.Artifact{
		Name: "m", Type: model.TypeMCP, Bundle: "b", Runtimes: []model.Runtime{model.RuntimeGemini},
		MCP: &model.MCPConfig{Transport: "stdio", Command: "stark-bq-mcp",
			Env: map[string]model.SecretRef{
				"C_KEY": {SecretRef: "c"}, "A_KEY": {SecretRef: "a"}, "B_KEY": {SecretRef: "b"},
			}},
	}
	r1, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	r2, _, _ := New().Render(bundleWith(a))
	b1, _ := find(r1, "settings.json")
	b2, _ := find(r2, "settings.json")
	if b1 != b2 {
		t.Fatal("multi-key env must be deterministic")
	}
	ia, ib, ic := strings.Index(b1, "A_KEY"), strings.Index(b1, "B_KEY"), strings.Index(b1, "C_KEY")
	if ia < 0 || ia > ib || ib > ic {
		t.Fatalf("env keys not in sorted order:\n%s", b1)
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

func TestGoldenGeminiCommand(t *testing.T) {
	a := &model.Artifact{
		Name: "review", Type: model.TypeCommand, Bundle: "stark-review",
		Description: "PR review command.", Version: "0.7.0",
		ArgumentHint: "[PR_NUMBER]", Body: "Review {{args}}.\n",
		Runtimes: []model.Runtime{model.RuntimeGemini},
	}
	files, _, _ := New().Render(bundleWith(a))
	assertGolden(t, "command.golden", files[0].Content)
}

func TestGoldenGeminiSkill(t *testing.T) {
	a := &model.Artifact{
		Name: "stark-review", Type: model.TypeSkill, Bundle: "stark-review",
		Description: "PR review.", Version: "0.7.0",
		Body:     "Review carefully.\n",
		Runtimes: []model.Runtime{model.RuntimeGemini},
	}
	files, _, _ := New().Render(bundleWith(a))
	assertGolden(t, "skill.golden", files[0].Content)
}
