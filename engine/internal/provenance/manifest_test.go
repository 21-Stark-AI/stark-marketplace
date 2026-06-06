package provenance

import (
	"encoding/json"
	"testing"
)

func TestComputeIsDeterministic(t *testing.T) {
	files := map[string][]byte{
		"dist/claude/stark-gh/plugin.json": []byte("a"),
		"index.json":                       []byte("b"),
	}
	targets := map[string]int{"claude": 1, "codex": 2}
	m1 := Compute(targets, files)
	m2 := Compute(targets, files)
	b1, _ := m1.Marshal()
	b2, _ := m2.Marshal()
	if string(b1) != string(b2) {
		t.Fatal("manifest must be byte-identical for identical inputs")
	}
}

func TestComputeDigestsSorted(t *testing.T) {
	files := map[string][]byte{"z.json": []byte("z"), "a.json": []byte("a")}
	m := Compute(map[string]int{}, files)
	if len(m.Files) != 2 || m.Files[0].Path != "a.json" || m.Files[1].Path != "z.json" {
		t.Fatalf("files must be sorted by path: %+v", m.Files)
	}
	// digest is sha256 hex (64 chars)
	if len(m.Files[0].Digest) != 64 {
		t.Fatalf("expected sha256 hex digest, got %q", m.Files[0].Digest)
	}
}

func TestMarshalIsSortedJSON(t *testing.T) {
	m := Compute(map[string]int{"gemini": 3, "claude": 1}, map[string][]byte{})
	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var probe BuildManifest
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if probe.TargetVersions["claude"] != 1 || probe.TargetVersions["gemini"] != 3 {
		t.Fatalf("target versions round-trip failed: %+v", probe.TargetVersions)
	}
}
