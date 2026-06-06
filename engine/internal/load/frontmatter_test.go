package load

import "testing"

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
