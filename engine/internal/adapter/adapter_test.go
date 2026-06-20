package adapter

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

type fakeTarget struct{}

func (fakeTarget) Runtime() model.Runtime { return model.RuntimeClaude }
func (fakeTarget) Version() string        { return "fake@1" }
func (fakeTarget) Render(b *model.Bundle) ([]OutputFile, []Finding, error) {
	return []OutputFile{{Path: "skills/x/SKILL.md", Content: []byte("hi\n")}},
		[]Finding{{Where: "demo/x", Level: "warn", Msg: "hi"}}, nil
}

func TestTargetInterfaceSatisfied(t *testing.T) {
	var tgt Target = fakeTarget{}
	files, findings, err := tgt.Render(&model.Bundle{Name: "demo"})
	if err != nil || len(files) != 1 || files[0].Path != "skills/x/SKILL.md" {
		t.Fatalf("unexpected: %v %v", files, err)
	}
	if len(findings) != 1 || findings[0].Level != "warn" || findings[0].Where != "demo/x" {
		t.Fatalf("findings wrong: %+v", findings)
	}
	if tgt.Version() != "fake@1" || tgt.Runtime() != model.RuntimeClaude {
		t.Fatal("metadata accessors wrong")
	}
}

func TestSortFilesDeterministic(t *testing.T) {
	in := []OutputFile{{Path: "b"}, {Path: "a"}, {Path: "c"}}
	SortFiles(in)
	if in[0].Path != "a" || in[1].Path != "b" || in[2].Path != "c" {
		t.Fatalf("not sorted: %+v", in)
	}
}
