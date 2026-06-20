package importer

import (
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
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

// Skills subset imports exactly the requested skills, in order, and nothing else.
func TestImportSkillsSubset(t *testing.T) {
	res, err := Import(Options{
		From:   "testdata/stark-skills",
		Bundle: "demo-skills",
		Skills: []string{"demo-review"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if findArtifact(res.Bundle, "demo-review") == nil {
		t.Fatal("demo-review should be imported")
	}
	if findArtifact(res.Bundle, "demo-release") != nil {
		t.Fatal("demo-release must NOT be imported (not in subset)")
	}
	if len(res.Bundle.Artifacts) != 1 {
		t.Fatalf("subset must import exactly 1 artifact, got %d", len(res.Bundle.Artifacts))
	}
}

// A typo'd/missing skill name in the subset is a hard error (fail-closed).
func TestImportSubsetMissingSkillErrors(t *testing.T) {
	if _, err := Import(Options{
		From: "testdata/stark-skills", Bundle: "demo-skills",
		Skills: []string{"does-not-exist"},
	}); err == nil {
		t.Fatal("requesting a missing skill must error")
	}
}

// The subset is imported in the order given (not sorted), and a duplicate name is a
// hard error (fail-closed — avoids writing a bundle with duplicate artifacts).
func TestImportSubsetOrderAndDuplicates(t *testing.T) {
	res, err := Import(Options{
		From: "testdata/stark-skills", Bundle: "demo-skills",
		Skills: []string{"demo-release", "demo-review"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bundle.Artifacts) != 2 ||
		res.Bundle.Artifacts[0].Name != "demo-release" ||
		res.Bundle.Artifacts[1].Name != "demo-review" {
		t.Fatalf("subset order not preserved: %v",
			[]string{res.Bundle.Artifacts[0].Name, res.Bundle.Artifacts[1].Name})
	}
	if _, err := Import(Options{
		From: "testdata/stark-skills", Bundle: "demo-skills",
		Skills: []string{"demo-review", "demo-review"},
	}); err == nil {
		t.Fatal("duplicate skill in the subset must error")
	}
}

// A bundle with no matching plugins/<bundle> pulls only skills — the stark-gh plugin
// must NOT leak in (regression guard for the de-hardcoded plugin path).
func TestImportPluginDecoupledFromSkillBundle(t *testing.T) {
	res, err := Import(Options{From: "testdata/stark-skills", Bundle: "demo-skills"})
	if err != nil {
		t.Fatal(err)
	}
	if findArtifact(res.Bundle, "pr-open") != nil || findArtifact(res.Bundle, "gh") != nil {
		t.Fatal("stark-gh plugin artifacts must not appear in a non-stark-gh bundle")
	}
}
