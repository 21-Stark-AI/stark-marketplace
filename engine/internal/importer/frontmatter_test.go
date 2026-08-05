package importer

import (
	"bytes"
	"testing"
)

func TestSplitFrontmatterIgnoresIndentedDelimiterInBlockScalar(t *testing.T) {
	src := "---\nname: x\noverrides:\n  codex:\n    body: |\n      before\n      ---\n      after\n---\nbase body\n"
	fm, body, err := splitFrontmatter([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if want := "      ---\n      after\n"; !bytes.Contains(fm, []byte(want)) {
		t.Fatalf("frontmatter ended inside block scalar:\n%s", fm)
	}
	if body != "base body\n" {
		t.Fatalf("body = %q, want base body", body)
	}
}
