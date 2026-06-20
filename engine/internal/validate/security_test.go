package validate

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func mcp(cmd string, args ...string) *model.Artifact {
	return &model.Artifact{Name: "m", Type: model.TypeMCP, Runtimes: model.AllRuntimes(),
		MCP: &model.MCPConfig{Transport: "stdio", Command: cmd, Args: args}}
}

func TestCommandAllowlist(t *testing.T) {
	r := &Result{}
	checkSecurity(r, "demo/mcp/m", mcp("totally-evil-binary"))
	if !r.HasErrors() {
		t.Fatal("expected allowlist rejection")
	}
	r2 := &Result{}
	checkSecurity(r2, "demo/mcp/m", mcp("node", "server.js"))
	if r2.HasErrors() {
		t.Fatalf("node should be allowed: %+v", r2.Errors)
	}
}

func TestInlineEvalRejected(t *testing.T) {
	r := &Result{}
	checkSecurity(r, "demo/mcp/m", mcp("node", "-e", "doEvil()"))
	if !r.HasErrors() {
		t.Fatal("expected inline-eval rejection")
	}
}

func TestInlineCredWarnsWithoutAt(t *testing.T) {
	// `key=` and `--token=` carry a literal value — they must WARN even with no `@`.
	cases := [][]string{
		{"--api-base", "https://api.example.com?key=abc123"},
		{"--token=ghp_deadbeef"},
	}
	for i, args := range cases {
		r := &Result{}
		checkSecurity(r, "demo/mcp/m", mcp("node", args...))
		if len(r.Warnings) == 0 {
			t.Fatalf("case %d: expected inline-credential warning for %v", i, args)
		}
	}
}

func TestInlineEvalEqualsFormRejected(t *testing.T) {
	// node honors `--eval=CODE` as a single token; the exact-token match alone would
	// let it through. Splitting on `=` must still reject it.
	for _, args := range [][]string{{"--eval=doEvil()"}, {"-e=doEvil()"}, {"--eval=x"}} {
		r := &Result{}
		checkSecurity(r, "demo/mcp/m", mcp("node", args...))
		if !r.HasErrors() {
			t.Fatalf("expected inline-eval rejection for %v", args)
		}
	}
}

func TestCommandPathRejected(t *testing.T) {
	// path.Base("/usr/bin/node") == "node" (allowlisted), so the path-rejection at
	// the basename check is the ONLY guard against an allowlisted binary behind a path.
	for _, cmd := range []string{"/usr/bin/node", "../node", "./node"} {
		r := &Result{}
		checkSecurity(r, "demo/mcp/m", mcp(cmd))
		if !r.HasErrors() {
			t.Fatalf("expected path rejection for %q", cmd)
		}
	}
}

func TestUnpinnedNpxWarns(t *testing.T) {
	r := &Result{}
	checkSecurity(r, "demo/mcp/m", mcp("npx", "-y", "some-mcp-server"))
	if len(r.Warnings) == 0 {
		t.Fatal("expected unpinned-npx warning")
	}
	r2 := &Result{}
	checkSecurity(r2, "demo/mcp/m", mcp("npx", "-y", "some-mcp-server@1.2.3"))
	if len(r2.Warnings) != 0 {
		t.Fatalf("pinned npx must not warn: %+v", r2.Warnings)
	}
}

func TestUserinfoCredWarnsOnURL(t *testing.T) {
	// The classic https://user:pass@host leak in mcp.url must warn.
	a := &model.Artifact{Name: "m", Type: model.TypeMCP, Runtimes: model.AllRuntimes(),
		MCP: &model.MCPConfig{Transport: "http", URL: "https://user:secret@api.example.com"}}
	r := &Result{}
	checkSecurity(r, "demo/mcp/m", a)
	if len(r.Warnings) == 0 {
		t.Fatal("expected userinfo-credential warning on URL")
	}
}

func TestBarePasswordWarns(t *testing.T) {
	r := &Result{}
	checkSecurity(r, "demo/mcp/m", mcp("node", "--conn", "host=db user=admin password=hunter2"))
	if len(r.Warnings) == 0 {
		t.Fatal("expected bare password= inline-credential warning")
	}
}

func TestSecurityChecksRunRegardlessOfTransport(t *testing.T) {
	// transport:http with a non-allowlisted command + an inline-eval flag must still
	// be gated — the highest-trust checks must not hinge on a self-declared transport.
	a := &model.Artifact{Name: "m", Type: model.TypeMCP, Runtimes: model.AllRuntimes(),
		MCP: &model.MCPConfig{Transport: "http", Command: "evil-bin", Args: []string{"--eval=x"}}}
	r := &Result{}
	checkSecurity(r, "demo/mcp/m", a)
	if !r.HasErrors() {
		t.Fatal("expected command-allowlist + inline-eval gating regardless of transport")
	}
}
