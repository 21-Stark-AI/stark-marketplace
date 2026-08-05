package load

import (
	"bytes"
	"testing"
)

func TestSplitFrontmatter(t *testing.T) {
	src := "---\nname: x\n---\nbody line\n"
	fm, body, err := splitFrontmatter([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(fm) != "name: x\n" {
		t.Fatalf("fm = %q", fm)
	}
	if body != "body line\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestSplitFrontmatterMissing(t *testing.T) {
	if _, _, err := splitFrontmatter([]byte("no frontmatter")); err == nil {
		t.Fatal("expected error when frontmatter missing")
	}
}

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
