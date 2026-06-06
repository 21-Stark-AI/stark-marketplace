package aggregate

import (
	"strings"
	"testing"
)

func TestMergeSortsByBundleName(t *testing.T) {
	secs := []Section{
		{Bundle: "zzz", Name: "b", Content: "B body\n"},
		{Bundle: "aaa", Name: "a", Content: "A body\n"},
	}
	out := Merge(secs)
	ai := strings.Index(out, "stark:begin aaa/a@")
	zi := strings.Index(out, "stark:begin zzz/b@")
	if ai < 0 || zi < 0 || ai > zi {
		t.Fatalf("sections must be sorted by <bundle>/<name>:\n%s", out)
	}
}

func TestMergeWrapsEachSectionInSentinels(t *testing.T) {
	out := Merge([]Section{{Bundle: "stark-review", Name: "x", Content: "hello\n"}})
	if !strings.Contains(out, "<!-- stark:begin stark-review/x@") {
		t.Fatalf("missing begin sentinel:\n%s", out)
	}
	if !strings.Contains(out, "<!-- stark:end stark-review/x -->") {
		t.Fatalf("missing end sentinel:\n%s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("missing content:\n%s", out)
	}
}

func TestMergeEmptyIsEmpty(t *testing.T) {
	if Merge(nil) != "" {
		t.Fatal("empty input must yield empty output")
	}
}
