package adapter_test

import (
	"sort"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/codex"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/gemini"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/load"
)

// emitAll runs the canonical bundle-level Render (CC-1) over every catalog bundle.
// The target resolves bodies via merge.Resolve internally — the caller does no
// fence.Strip and passes no body.
func emitAll(t *testing.T, tgt adapter.Target) map[string]string {
	t.Helper()
	cat, err := load.Load("../../../catalog")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, b := range cat.Bundles {
		files, _, err := tgt.Render(b)
		if err != nil {
			t.Fatalf("%s render on %s: %v", b.Name, tgt.Runtime(), err)
		}
		for _, f := range files {
			out[f.Path] += string(f.Content) // shared files accumulate
		}
	}
	return out
}

func TestThreeRuntimeEmitDeterministic(t *testing.T) {
	for _, tgt := range []adapter.Target{codex.New(), gemini.New()} {
		first := emitAll(t, tgt)
		second := emitAll(t, tgt)
		if len(first) != len(second) {
			t.Fatalf("%s: file count differs", tgt.Runtime())
		}
		paths := make([]string, 0, len(first))
		for p := range first {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			if first[p] != second[p] {
				t.Fatalf("%s: %s not deterministic", tgt.Runtime(), p)
			}
		}
	}
}

func TestCodexEmitsSomethingForSeed(t *testing.T) {
	out := emitAll(t, codex.New())
	if len(out) == 0 {
		t.Fatal("codex produced no output for the seed catalog")
	}
}
