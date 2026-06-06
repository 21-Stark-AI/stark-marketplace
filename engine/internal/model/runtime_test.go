package model

import "testing"

func TestParseRuntime(t *testing.T) {
	r, err := ParseRuntime("claude")
	if err != nil || r != RuntimeClaude {
		t.Fatalf("got %q err %v", r, err)
	}
	if _, err := ParseRuntime("bogus"); err == nil {
		t.Fatal("expected error for unknown runtime")
	}
}

func TestAllRuntimes(t *testing.T) {
	if len(AllRuntimes()) != 3 {
		t.Fatalf("want 3 runtimes, got %d", len(AllRuntimes()))
	}
}
