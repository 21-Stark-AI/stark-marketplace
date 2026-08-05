package codex

import (
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/adapter"
	"github.com/21StarkCom/bifrost/engine/internal/model"
)

func TestCodexEmitsMCPConfigToml(t *testing.T) {
	a := &model.Artifact{
		Name: "bigquery", Type: model.TypeMCP, Bundle: "stark-data",
		Description: "BQ MCP.", Version: "1.2.0",
		Runtimes: []model.Runtime{model.RuntimeCodex},
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
	body, ok := findFile(files, ".codex/config.toml")
	if !ok {
		t.Fatalf("expected .codex/config.toml; got %v", files)
	}
	// go-toml/v2 emits literal (single-quoted) strings for values with no special
	// chars — valid, deterministic TOML. The Codex MCP key is [mcp_servers.<name>].
	for _, want := range []string{
		"[mcp_servers.bigquery]",
		`command = 'stark-bq-mcp'`,
		`args = ['--project', '${BQ_PROJECT}']`,
		`env_vars = ['BQ_PROJECT']`,
	} {
		if !contains(body, want) {
			t.Fatalf("config.toml missing %q in:\n%s", want, body)
		}
	}
}

func TestCodexMCPEnvMultiKeySorted(t *testing.T) {
	a := &model.Artifact{
		Name: "m", Type: model.TypeMCP, Bundle: "b", Runtimes: []model.Runtime{model.RuntimeCodex},
		MCP: &model.MCPConfig{Transport: "stdio", Command: "stark-bq-mcp",
			Env: map[string]model.SecretRef{
				"C_KEY": {SecretRef: "c"}, "A_KEY": {SecretRef: "a"}, "B_KEY": {SecretRef: "b"},
			}},
	}
	b1, _ := findFile(mustRender(t, a), ".codex/config.toml")
	b2, _ := findFile(mustRender(t, a), ".codex/config.toml")
	if b1 != b2 {
		t.Fatal("multi-key env must be deterministic")
	}
	ia, ib, ic := strings.Index(b1, "A_KEY"), strings.Index(b1, "B_KEY"), strings.Index(b1, "C_KEY")
	if ia < 0 || ia > ib || ib > ic {
		t.Fatalf("env keys not in sorted order:\n%s", b1)
	}
}

func TestCodexEmitsHTTPMCPConfigToml(t *testing.T) {
	a := &model.Artifact{
		Name: "remote", Type: model.TypeMCP, Bundle: "b", Runtimes: []model.Runtime{model.RuntimeCodex},
		MCP: &model.MCPConfig{Transport: "http", URL: "https://mcp.example.com/v1"},
	}
	body, ok := findFile(mustRender(t, a), ".codex/config.toml")
	if !ok || !contains(body, `url = 'https://mcp.example.com/v1'`) {
		t.Fatalf("HTTP MCP URL missing:\n%s", body)
	}
	for _, forbidden := range []string{"command =", "args =", "env_vars ="} {
		if contains(body, forbidden) {
			t.Fatalf("HTTP MCP config contains stdio-only field %q:\n%s", forbidden, body)
		}
	}
}

func TestCodexRejectsCanonicalEnvOnHTTPMCP(t *testing.T) {
	a := &model.Artifact{
		Name: "remote", Type: model.TypeMCP, Bundle: "b", Runtimes: []model.Runtime{model.RuntimeCodex},
		MCP: &model.MCPConfig{
			Transport: "http", URL: "https://mcp.example.com/v1",
			Env: map[string]model.SecretRef{"TOKEN": {SecretRef: "remote-token"}},
		},
	}
	_, _, err := New().Render(bundleWith(a))
	if err == nil || !strings.Contains(err.Error(), "cannot map canonical env to HTTP authentication") {
		t.Fatalf("expected fail-closed HTTP auth error, got %v", err)
	}
}

func mustRender(t *testing.T, a *model.Artifact) []adapter.OutputFile {
	t.Helper()
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	return files
}
