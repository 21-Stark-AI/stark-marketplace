package digest

import "testing"

func TestOutputDigestStableAndOrderInsensitive(t *testing.T) {
	files := map[string][]byte{
		"skills/x/SKILL.md": []byte("a\n"),
		"commands/x.md":     []byte("b\n"),
	}
	reordered := map[string][]byte{
		"commands/x.md":     []byte("b\n"),
		"skills/x/SKILL.md": []byte("a\n"),
	}
	if Output(files, "claude@1") != Output(reordered, "claude@1") {
		t.Fatal("output digest must be order-insensitive over file map")
	}
}

func TestOutputDigestChangesWithAdapterVersion(t *testing.T) {
	files := map[string][]byte{"a": []byte("x")}
	if Output(files, "claude@1") == Output(files, "claude@2") {
		t.Fatal("adapter version must participate in the output digest")
	}
}

func TestOutputDigestChangesWithBytes(t *testing.T) {
	if Output(map[string][]byte{"a": []byte("x")}, "claude@1") ==
		Output(map[string][]byte{"a": []byte("y")}, "claude@1") {
		t.Fatal("byte change must change output digest")
	}
}
