package main

import (
	"strings"
	"testing"
)

func TestInfoRendersSupportAndClosure(t *testing.T) {
	out, code := renderInfo(
		"../../internal/indexio/testdata/index.json",
		"../../internal/indexio/testdata/bundles",
		"stark-gh", "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "stark-gh") || !strings.Contains(out, "pr-open") ||
		!strings.Contains(out, "gh") {
		t.Fatalf("info missing artifacts: %s", out)
	}
	if !strings.Contains(out, "codex") || !strings.Contains(out, "native") {
		t.Fatalf("info missing support matrix: %s", out)
	}
}

func TestInfoSingleArtifact(t *testing.T) {
	out, code := renderInfo(
		"../../internal/indexio/testdata/index.json",
		"../../internal/indexio/testdata/bundles",
		"stark-gh", "gh")
	if code != 0 || !strings.Contains(out, "mcp") {
		t.Fatalf("single-artifact info wrong (code %d): %s", code, out)
	}
}
