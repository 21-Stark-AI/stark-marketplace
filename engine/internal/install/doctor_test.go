package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/installplan"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestDoctorDetectsBrokenAndIntact(t *testing.T) {
	dest := t.TempDir()
	p := &installplan.Plan{Runtime: model.RuntimeCodex, Steps: []installplan.Step{
		{Bundle: "rev", Name: "session", Type: model.TypeSkill, Files: []installplan.AdaptedFile{
			{Path: ".agents/skills/session/SKILL.md", Kind: "file", Payload: "session\n"}}},
	}}
	res, err := Install(dest, p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// healthy
	report, err := Doctor(dest, res.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Broken) != 0 {
		t.Fatalf("expected healthy, got broken: %+v", report.Broken)
	}
	// break it: delete the managed file
	os.Remove(filepath.Join(dest, ".agents/skills/session/SKILL.md"))
	report2, _ := Doctor(dest, res.ManifestPath)
	if len(report2.Broken) != 1 {
		t.Fatalf("expected 1 broken, got %+v", report2.Broken)
	}
}
