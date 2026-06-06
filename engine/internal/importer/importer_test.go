package importer

import (
	"strings"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func findArtifact(b *model.Bundle, name string) *model.Artifact {
	for _, a := range b.Artifacts {
		if a.Name == name {
			return a
		}
	}
	return nil
}

func TestImportSkillsFromFixture(t *testing.T) {
	res, err := Import(Options{
		From:   "testdata/stark-skills",
		Bundle: "demo-skills",
	})
	if err != nil {
		t.Fatal(err)
	}
	rv := findArtifact(res.Bundle, "demo-review")
	if rv == nil {
		t.Fatal("demo-review not imported")
	}
	if rv.Type != model.TypeSkill {
		t.Fatalf("type = %q, want skill", rv.Type)
	}
	// carried fields
	if rv.ArgumentHint == "" || rv.Model != "opus[1m]" {
		t.Fatalf("carry failed: hint=%q model=%q", rv.ArgumentHint, rv.Model)
	}
	if rv.DisableModelInvocation != false {
		t.Fatal("disable-model-invocation should carry false")
	}
	// body preserved verbatim (starts with the source first line)
	if !strings.HasPrefix(rv.Body, "Single-agent PR review path.") {
		t.Fatalf("body not preserved: %q", rv.Body[:min(40, len(rv.Body))])
	}
	// revision/revision_date dropped (no model field for them)
	for _, n := range res.Notes {
		if n.Field == "revision" {
			goto dropnoted
		}
	}
	t.Fatal("expected a note that revision/* was dropped")
dropnoted:
	// release skill carries disable-model-invocation: true + allowed-tools absent
	rl := findArtifact(res.Bundle, "demo-release")
	if rl == nil || rl.DisableModelInvocation != true {
		t.Fatalf("release skill mapping wrong: %+v", rl)
	}
}
