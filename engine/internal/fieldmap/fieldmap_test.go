package fieldmap

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestActionsMatchSpec62(t *testing.T) {
	cases := []struct {
		field string
		rt    model.Runtime
		want  Action
	}{
		{"model", model.RuntimeClaude, ActionCarry},
		{"model", model.RuntimeCodex, ActionMap},
		{"model", model.RuntimeGemini, ActionDrop},
		{"argument-hint", model.RuntimeCodex, ActionDerive},
		{"argument-hint", model.RuntimeGemini, ActionDerive},
		{"disable-model-invocation", model.RuntimeCodex, ActionDrop},
		{"disable-model-invocation", model.RuntimeGemini, ActionDrop},
		{"allowed-tools", model.RuntimeCodex, ActionBestEffort},
		{"allowed-tools", model.RuntimeGemini, ActionDrop},
	}
	for _, c := range cases {
		if got := actionFor(c.field, c.rt); got != c.want {
			t.Errorf("actionFor(%s,%s) = %q, want %q", c.field, c.rt, got, c.want)
		}
	}
}

func TestUnknownFieldCarries(t *testing.T) {
	if got := actionFor("category", model.RuntimeClaude); got != ActionCarry {
		t.Fatalf("default should be carry, got %q", got)
	}
}
