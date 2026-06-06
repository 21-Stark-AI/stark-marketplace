package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteBundleLayout(t *testing.T) {
	res, err := Import(Options{From: "testdata/stark-skills", Bundle: "stark-gh"})
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := WriteBundle(res, dst); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dst, "stark-gh")
	must := []string{
		"bundle.yaml",
		"commands/pr-open.md",
		"commands/cleanup.md",
		"mcp/gh.yaml",
		"IMPORT-NOTES.md",
	}
	for _, p := range must {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
	notes, _ := os.ReadFile(filepath.Join(root, "IMPORT-NOTES.md"))
	if len(notes) == 0 || !filepath.IsAbs(dst) {
		t.Fatal("IMPORT-NOTES.md empty")
	}
}
