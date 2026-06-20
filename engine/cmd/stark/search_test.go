package main

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/indexio"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func testIndex(t *testing.T) *indexio.Index {
	t.Helper()
	idx, err := indexio.LoadIndex("../../internal/indexio/testdata/index.json")
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestSearchFiltersByTypeAndRuntime(t *testing.T) {
	idx := testIndex(t)
	got := searchIndex(idx, searchOpts{query: "pr", typ: "command", runtime: model.RuntimeCodex})
	if len(got) != 1 || got[0].Name != "pr-open" {
		t.Fatalf("search result wrong: %+v", got)
	}
}

func TestSearchExcludesDeprecatedByDefault(t *testing.T) {
	idx := &indexio.Index{SchemaVersion: 1, Artifacts: []indexio.Entry{
		{Name: "old", Type: model.TypeCommand, Bundle: "b", Maturity: model.MaturityDeprecated,
			Support: map[model.Runtime]model.SupportLevel{model.RuntimeClaude: model.SupportNative}},
	}}
	if got := searchIndex(idx, searchOpts{}); len(got) != 0 {
		t.Fatalf("deprecated should be hidden by default: %+v", got)
	}
	if got := searchIndex(idx, searchOpts{includeDeprecated: true}); len(got) != 1 {
		t.Fatal("includeDeprecated should surface it")
	}
}
