package capability

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestLevelsPerCorrectedMatrix(t *testing.T) {
	cases := []struct {
		typ  model.ArtifactType
		rt   model.Runtime
		want model.SupportLevel
	}{
		{model.TypeSkill, model.RuntimeCodex, model.SupportNative},
		{model.TypePrompt, model.RuntimeCodex, model.SupportNative},
		{model.TypeCommand, model.RuntimeCodex, model.SupportNative},
		{model.TypeAgent, model.RuntimeCodex, model.SupportEmulated},
		{model.TypeMCP, model.RuntimeCodex, model.SupportNative},
		{model.TypePrompt, model.RuntimeGemini, model.SupportNative},
		{model.TypeCommand, model.RuntimeGemini, model.SupportNative},
		{model.TypeSkill, model.RuntimeGemini, model.SupportEmulated},
		{model.TypeAgent, model.RuntimeGemini, model.SupportEmulated},
		{model.TypeMCP, model.RuntimeGemini, model.SupportNative},
		{model.TypeSkill, model.RuntimeClaude, model.SupportNative},
		{model.TypeAgent, model.RuntimeClaude, model.SupportNative},
	}
	for _, c := range cases {
		if got := Level(c.typ, c.rt); got != c.want {
			t.Errorf("Level(%s,%s) = %q, want %q", c.typ, c.rt, got, c.want)
		}
	}
}

func TestUnknownPairIsUnsupported(t *testing.T) {
	if got := Level("bogus", model.RuntimeCodex); got != model.SupportUnsupported {
		t.Fatalf("unknown type should be unsupported, got %q", got)
	}
}

func TestVersionIsStable(t *testing.T) {
	if Version == "" {
		t.Fatal("capability matrix must declare a version")
	}
}
