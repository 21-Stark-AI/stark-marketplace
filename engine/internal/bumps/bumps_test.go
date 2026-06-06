package bumps

import "testing"

func TestNoViolationsWhenEmptyPrevious(t *testing.T) {
	v := Check(nil, map[string]Current{
		"demo/rev": {Version: "1.0.0", SourceDigest: "sha256:aaa"},
	})
	if len(v) != 0 {
		t.Fatalf("empty previous must skip: %v", v)
	}
}

func TestNoViolationWhenDigestUnchanged(t *testing.T) {
	prev := map[string]Previous{"demo/rev": {Version: "1.0.0", Digest: "sha256:aaa"}}
	cur := map[string]Current{"demo/rev": {Version: "1.0.0", SourceDigest: "sha256:aaa"}}
	if v := Check(prev, cur); len(v) != 0 {
		t.Fatalf("unchanged digest must pass: %v", v)
	}
}

func TestNoViolationWhenVersionBumped(t *testing.T) {
	prev := map[string]Previous{"demo/rev": {Version: "1.0.0", Digest: "sha256:aaa"}}
	cur := map[string]Current{"demo/rev": {Version: "1.1.0", SourceDigest: "sha256:bbb"}}
	if v := Check(prev, cur); len(v) != 0 {
		t.Fatalf("bumped version must pass: %v", v)
	}
}

func TestViolationWhenDigestChangedButVersionSame(t *testing.T) {
	prev := map[string]Previous{"demo/rev": {Version: "1.0.0", Digest: "sha256:aaa"}}
	cur := map[string]Current{"demo/rev": {Version: "1.0.0", SourceDigest: "sha256:bbb"}}
	v := Check(prev, cur)
	if len(v) != 1 || v[0].Key != "demo/rev" {
		t.Fatalf("expected 1 violation for demo/rev, got %v", v)
	}
}

func TestNewArtifactNotAViolation(t *testing.T) {
	prev := map[string]Previous{} // present-but-empty previous index
	cur := map[string]Current{"demo/new": {Version: "0.1.0", SourceDigest: "sha256:ccc"}}
	if v := Check(prev, cur); len(v) != 0 {
		t.Fatalf("brand-new artifact must pass: %v", v)
	}
}
