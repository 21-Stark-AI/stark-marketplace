package importer

import "testing"

// Real stark-skills frontmatter writes argument-hint unquoted with YAML-significant chars,
// which strict YAML rejects. decodeFrontmatter must fall back to a sanitized re-parse.
func TestDecodeFrontmatterSanitizesLooseArgumentHint(t *testing.T) {
	loose := []byte("name: rel\n" +
		"description: a release skill\n" +
		"argument-hint: [patch|minor|major] (optional — auto-detected if omitted)\n" +
		"model: sonnet\n")
	raw, sanitized, err := decodeFrontmatter(loose)
	if err != nil {
		t.Fatalf("loose frontmatter should parse via sanitize fallback: %v", err)
	}
	if !sanitized {
		t.Fatal("expected sanitized=true for the loose argument-hint")
	}
	hint, ok := raw["argument-hint"].(string)
	if !ok || hint != "[patch|minor|major] (optional — auto-detected if omitted)" {
		t.Fatalf("argument-hint not recovered as a string: %v (%T)", raw["argument-hint"], raw["argument-hint"])
	}
	if raw["name"] != "rel" || raw["model"] != "sonnet" {
		t.Fatalf("other fields lost: %+v", raw)
	}
}

func TestDecodeFrontmatterStrictPathUnchanged(t *testing.T) {
	clean := []byte("name: rel\nargument-hint: \"[patch|minor|major]\"\n")
	raw, sanitized, err := decodeFrontmatter(clean)
	if err != nil {
		t.Fatal(err)
	}
	if sanitized {
		t.Fatal("already-valid frontmatter must not report sanitized")
	}
	if raw["argument-hint"] != "[patch|minor|major]" {
		t.Fatalf("argument-hint = %v", raw["argument-hint"])
	}
}
