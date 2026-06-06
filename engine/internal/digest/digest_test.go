package digest

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func base() *model.Artifact {
	return &model.Artifact{
		Name: "x", Type: model.TypeCommand, Description: "first", Version: "1.0.0",
		Tags: []string{"a"}, Summary: "s1", ArgumentHint: "[n]",
		Body: "body\n",
	}
}

func TestSourceDigestIgnoresDisplayMetadata(t *testing.T) {
	a := base()
	d1 := Source(a)

	b := base()
	b.Description = "second" // display only
	b.Tags = []string{"a", "b"}
	b.Summary = "s2"
	b.Version = "9.9.9" // version itself excluded from the bump-gate hash
	d2 := Source(b)

	if d1 != d2 {
		t.Fatalf("display-only edits must NOT change source digest: %s vs %s", d1, d2)
	}
}

func TestSourceDigestChangesOnBody(t *testing.T) {
	a := base()
	d1 := Source(a)
	b := base()
	b.Body = "DIFFERENT body\n"
	if Source(b) == d1 {
		t.Fatal("body change must change source digest")
	}
}

func TestSourceDigestChangesOnCanonicalField(t *testing.T) {
	a := base()
	d1 := Source(a)
	b := base()
	b.ArgumentHint = "[other]"
	if Source(b) == d1 {
		t.Fatal("argument-hint change must change source digest")
	}
}
