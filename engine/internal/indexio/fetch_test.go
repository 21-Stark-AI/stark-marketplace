package indexio

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestFetchIndexAndDetailViaContentsAPI(t *testing.T) {
	const wantToken = "ghs_testtoken"
	idxJSON := `{"schemaVersion":1,"artifacts":[
		{"name":"gh","type":"mcp","bundle":"stark-gh","version":"0.1.0",
		 "support":{"codex":"native"}}]}`
	detailJSON := `{"schemaVersion":1,
		"bundle":{"name":"stark-gh","version":"0.1.0","description":"x","maturity":"beta"},
		"artifacts":[{"name":"gh","type":"mcp","version":"0.1.0","runtimes":["codex"],
		 "support":{"codex":"native"},"diverged":false,
		 "outputs":{"codex":[{"path":"config.toml","kind":"mergeTOMLKey","key":"mcp_servers.gh"}]},
		 "mcp":{"transport":"stdio","command":"node","args":["x.js"]}}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "token "+wantToken {
			t.Errorf("auth header = %q", got)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Accept") != "application/vnd.github.raw" {
			t.Errorf("accept = %q", r.Header.Get("Accept"))
		}
		if r.URL.Query().Get("ref") != "main" {
			t.Errorf("ref = %q", r.URL.Query().Get("ref"))
		}
		switch r.URL.Path {
		case "/repos/GetEvinced/stark-marketplace/contents/dist/claude/index.json":
			w.Write([]byte(idxJSON))
		case "/repos/GetEvinced/stark-marketplace/contents/dist/claude/bundles/stark-gh.json":
			w.Write([]byte(detailJSON))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := &Fetcher{
		APIBase:  srv.URL,
		Owner:    "GetEvinced",
		Repo:     "stark-marketplace",
		Ref:      "main",
		BasePath: "dist/claude",
		Token:    wantToken,
		HTTP:     srv.Client(),
	}

	idx, err := f.FetchIndex()
	if err != nil {
		t.Fatal(err)
	}
	if idx.SchemaVersion != 1 || len(idx.Artifacts) != 1 {
		t.Fatalf("index wrong: %+v", idx)
	}
	d, err := f.FetchBundleDetail("stark-gh")
	if err != nil {
		t.Fatal(err)
	}
	a := d.Artifact("gh", model.TypeMCP)
	if a == nil || a.Outputs[model.RuntimeCodex][0].Key != "mcp_servers.gh" {
		t.Fatalf("detail wrong: %+v", d)
	}
}

func TestFetchRejectsBadSchemaVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"schemaVersion":99,"artifacts":[]}`))
	}))
	defer srv.Close()
	f := &Fetcher{APIBase: srv.URL, Owner: "o", Repo: "r", Ref: "main",
		BasePath: "dist/claude", Token: "t", HTTP: srv.Client()}
	if _, err := f.FetchIndex(); err == nil || !strings.Contains(err.Error(), "schemaVersion") {
		t.Fatalf("want schemaVersion error, got %v", err)
	}
}

func TestResolveTokenPrefersEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-tok")
	if tok := resolveToken(func() (string, error) { return "gh-tok", nil }); tok != "env-tok" {
		t.Fatalf("env token should win when set, got %q", tok)
	}
	t.Setenv("GITHUB_TOKEN", "")
	if tok := resolveToken(func() (string, error) { return "gh-tok", nil }); tok != "gh-tok" {
		t.Fatalf("gh fallback failed, got %q", tok)
	}
}
