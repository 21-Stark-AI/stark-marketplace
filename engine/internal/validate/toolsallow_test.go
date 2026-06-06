package validate

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func agentWithTools(tools ...string) *model.Artifact {
	return &model.Artifact{Name: "a", Type: model.TypeAgent,
		Runtimes: model.AllRuntimes(), Tools: tools}
}

func TestAgentToolsAllowlistWarnsUnknown(t *testing.T) {
	r := &Result{}
	checkAgentTools(r, "demo/agent/a", agentWithTools("Bash", "MysteryTool"))
	if len(r.Warnings) != 1 {
		t.Fatalf("want 1 warning for unknown tool, got %d: %+v", len(r.Warnings), r.Warnings)
	}
	if r.HasErrors() {
		t.Fatal("unknown tool is a warning (surfaced), not an error")
	}
}

func TestAgentToolsAllKnownNoWarn(t *testing.T) {
	r := &Result{}
	checkAgentTools(r, "demo/agent/a", agentWithTools("Bash", "Read", "Edit", "Grep"))
	if len(r.Warnings) != 0 {
		t.Fatalf("known tools should not warn: %+v", r.Warnings)
	}
}
