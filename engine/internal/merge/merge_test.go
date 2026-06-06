package merge

import (
	"reflect"
	"testing"
)

func TestDeepMergeScalarsReplaceMapsMergeArraysReplace(t *testing.T) {
	base := map[string]any{
		"model": "opus",
		"tags":  []any{"a", "b"},
		"meta":  map[string]any{"x": 1, "y": 2},
	}
	patch := map[string]any{
		"model": "gemini-2.5-pro",       // scalar replace
		"tags":  []any{"c"},             // array replace wholesale
		"meta":  map[string]any{"y": 9}, // map merge
	}
	got := deepMerge(base, patch)
	want := map[string]any{
		"model": "gemini-2.5-pro",
		"tags":  []any{"c"},
		"meta":  map[string]any{"x": 1, "y": 9},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	// base must not be mutated
	if base["model"] != "opus" {
		t.Fatal("deepMerge mutated base")
	}
}
