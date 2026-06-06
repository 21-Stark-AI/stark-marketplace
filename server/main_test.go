package main

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() fs.FS {
	return fstest.MapFS{
		"index.html":         {Data: []byte("<!doctype html><title>stark-marketplace</title>")},
		"index.json":         {Data: []byte(`{"schemaVersion":1,"bundles":[]}`)},
		"assets/index.abc.js": {Data: []byte("console.log('hi')")},
		"bundles/stark-gh.json": {Data: []byte(`{"bundle":"stark-gh"}`)},
	}
}

func do(t *testing.T, h http.Handler, method, target string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestHealthz(t *testing.T) {
	resp := do(t, handler(testFS()), http.MethodGet, "/healthz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("healthz Cache-Control = %q, want no-store", cc)
	}
}

func TestServesShellAtRoot(t *testing.T) {
	resp := do(t, handler(testFS()), http.MethodGet, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("shell Cache-Control = %q, want no-cache", cc)
	}
	if ct := resp.Header.Get("Content-Type"); ct == "" || ct[:9] != "text/html" {
		t.Fatalf("shell Content-Type = %q, want text/html", ct)
	}
}

func TestAssetIsImmutable(t *testing.T) {
	resp := do(t, handler(testFS()), http.MethodGet, "/assets/index.abc.js")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Fatalf("asset Cache-Control = %q, want immutable", cc)
	}
}

func TestDataFilesAreNoCache(t *testing.T) {
	for _, target := range []string{"/index.json", "/bundles/stark-gh.json"} {
		resp := do(t, handler(testFS()), http.MethodGet, target)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", target, resp.StatusCode)
		}
		if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
			t.Fatalf("%s Cache-Control = %q, want no-cache", target, cc)
		}
	}
}

func TestUnknownPathFallsBackToShell(t *testing.T) {
	resp := do(t, handler(testFS()), http.MethodGet, "/bundle/stark-gh")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (SPA shell)", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) == "" || string(body)[:9] != "<!doctype" {
		t.Fatalf("fallback did not serve the shell: %q", body)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("fallback Cache-Control = %q, want no-cache", cc)
	}
}

func TestTraversalIsContained(t *testing.T) {
	// A traversal attempt must never escape the FS. http.ServeFileFS rejects any
	// request whose URL path contains ".." with 400 before serving — so the
	// attempt is contained (rejected), and crucially never leaks a file outside
	// the root. (Hash-routed SPA paths never contain "..", so this only ever
	// fires on hostile input.)
	resp := do(t, handler(testFS()), http.MethodGet, "/../../etc/passwd")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (contained)", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "root:") {
		t.Fatalf("traversal leaked file contents: %q", body)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	resp := do(t, handler(testFS()), http.MethodPost, "/")
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
	if allow := resp.Header.Get("Allow"); allow != "GET, HEAD" {
		t.Fatalf("Allow = %q, want GET, HEAD", allow)
	}
}

func TestHeadOK(t *testing.T) {
	resp := do(t, handler(testFS()), http.MethodHead, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
