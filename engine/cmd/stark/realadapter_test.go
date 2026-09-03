package main

import (
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/aggregate"
	"github.com/21StarkCom/bifrost/engine/internal/install"
)

// The real adapter renders sentinel emulation blocks with digest-bearing markers
// (`<!-- stark:begin id@<digest> -->`). sentinelBody must recover the CLEAN inner body so
// install.MergeSentinel wraps it exactly once — otherwise the markers leak and a second install
// double-wraps and corrupts GEMINI.md/AGENTS.md.
func TestSentinelBodyStripsRenderMarkersNoDoubleWrap(t *testing.T) {
	rendered := aggregate.Merge([]aggregate.Section{{Bundle: "multi", Name: "agentmd", Content: "agent role line\n"}})
	if !strings.Contains(rendered, "@") {
		t.Fatalf("precondition: render markers should carry a digest:\n%s", rendered)
	}
	body, err := sentinelBody(rendered, "multi/agentmd")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "stark:begin") || strings.Contains(body, "stark:end") {
		t.Fatalf("markers leaked into stripped body: %q", body)
	}
	if !strings.Contains(body, "agent role line") {
		t.Fatalf("body content lost: %q", body)
	}
	// install wraps the clean body exactly once, and a re-merge is idempotent
	once, _, err := install.MergeSentinel(nil, "multi/agentmd", body)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(once), "stark:begin multi/agentmd"); n != 1 {
		t.Fatalf("expected exactly one begin marker, got %d:\n%s", n, once)
	}
	twice, _, _ := install.MergeSentinel(once, "multi/agentmd", body)
	if string(once) != string(twice) {
		t.Fatalf("sentinel re-merge not idempotent")
	}
	if _, err := sentinelBody(rendered, "multi/missing"); err == nil {
		t.Fatal("sentinelBody must error when the section id is absent")
	}
}
