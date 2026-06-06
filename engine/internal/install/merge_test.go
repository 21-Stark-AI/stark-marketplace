package install

import (
	"strings"
	"testing"
)

const existingTOML = `# user's own comment — must survive
log_level = "info"

[mcp_servers.other]
command = "keep-me"

# trailing user note
`

func TestMergeTOMLPreservesForeignContent(t *testing.T) {
	managed := "command = \"node\"\nargs = [\"gh.js\"]\n"
	out, action, err := MergeTOMLKey([]byte(existingTOML), "mcp_servers.gh", managed)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# user's own comment") || !strings.Contains(s, "[mcp_servers.other]") ||
		!strings.Contains(s, "keep-me") || !strings.Contains(s, "# trailing user note") {
		t.Fatalf("foreign content lost:\n%s", s)
	}
	if !strings.Contains(s, "[mcp_servers.gh]") || !strings.Contains(s, "gh.js") {
		t.Fatalf("managed table not inserted:\n%s", s)
	}
	if action == "" {
		t.Fatal("action must be reported")
	}
}

func TestMergeTOMLReplacesManagedTableIdempotent(t *testing.T) {
	managed := "command = \"node\"\n"
	once, _, _ := MergeTOMLKey([]byte(existingTOML), "mcp_servers.gh", managed)
	twice, _, _ := MergeTOMLKey(once, "mcp_servers.gh", managed)
	if string(once) != string(twice) {
		t.Fatalf("merge not idempotent:\n--once--\n%s\n--twice--\n%s", once, twice)
	}
}

const parentTOML = `[mcp_servers]
note = "parent table the user wrote"

[other]
x = 1
`

func TestMergeTOMLParentTableRoundTrips(t *testing.T) {
	managed := "command = \"node\"\n"
	once, action, err := MergeTOMLKey([]byte(parentTOML), "mcp_servers.gh", managed)
	if err != nil {
		t.Fatal(err)
	}
	if action != "insert" {
		t.Fatalf("action = %s", action)
	}
	s := string(once)
	if !strings.Contains(s, `note = "parent table the user wrote"`) || !strings.Contains(s, "[other]") {
		t.Fatalf("parent/other table lost:\n%s", s)
	}
	if !strings.Contains(s, "[mcp_servers.gh]") {
		t.Fatalf("child table not inserted:\n%s", s)
	}
	twice, _, _ := MergeTOMLKey(once, "mcp_servers.gh", managed)
	if string(once) != string(twice) {
		t.Fatalf("parent-table merge not idempotent:\n--once--\n%s\n--twice--\n%s", once, twice)
	}
}

const subtableTOML = `# user note
log_level = "info"

[mcp_servers.gh]
command = "node"

[mcp_servers.gh.env]
GITHUB_TOKEN = "x"

[mcp_servers.other]
command = "keep-me"
`

func TestMergeTOMLSubtableStaysWithManaged(t *testing.T) {
	managed := "command = \"node\"\n"
	out, action, err := MergeTOMLKey([]byte(subtableTOML), "mcp_servers.gh", managed)
	if err != nil {
		t.Fatal(err)
	}
	if action != "replace" {
		t.Fatalf("action = %s", action)
	}
	s := string(out)
	if strings.Contains(s, "[mcp_servers.gh.env]") || strings.Contains(s, "GITHUB_TOKEN") {
		t.Fatalf("orphaned subtable not removed with managed table:\n%s", s)
	}
	if !strings.Contains(s, "[mcp_servers.other]") || !strings.Contains(s, "keep-me") ||
		!strings.Contains(s, "# user note") {
		t.Fatalf("foreign content lost:\n%s", s)
	}
	if !strings.Contains(s, "[mcp_servers.gh]") {
		t.Fatalf("managed table missing:\n%s", s)
	}
	twice, _, _ := MergeTOMLKey(out, "mcp_servers.gh", managed)
	if string(out) != string(twice) {
		t.Fatalf("subtable merge not idempotent:\n--once--\n%s\n--twice--\n%s", out, twice)
	}
}

func TestMergeTOMLRefusesPayloadWithHeader(t *testing.T) {
	bad := "[mcp_servers.evil]\ncommand = \"x\"\n"
	if _, _, err := MergeTOMLKey([]byte(existingTOML), "mcp_servers.gh", bad); err == nil {
		t.Fatal("expected refusal: managed payload contains a FOREIGN table header")
	}
}

// A real runtime target renders the managed payload as a complete [dottedKey] block with its
// own child subtable (env). MergeTOMLKey must splice it whole, preserve foreign content, and
// stay idempotent across a re-merge (the region findTable excises includes the subtable).
func TestMergeTOMLAcceptsOwnBlockWithSubtable(t *testing.T) {
	block := "[mcp_servers.gh]\ncommand = 'node'\nargs = ['gh.js']\n\n[mcp_servers.gh.env]\nGITHUB_TOKEN = '${GITHUB_TOKEN}'\n"
	out, action, err := MergeTOMLKey([]byte(existingTOML), "mcp_servers.gh", block)
	if err != nil {
		t.Fatal(err)
	}
	if action != "insert" {
		t.Fatalf("action = %s", action)
	}
	s := string(out)
	if !strings.Contains(s, "[mcp_servers.gh.env]") || !strings.Contains(s, "GITHUB_TOKEN") {
		t.Fatalf("own subtable not carried through:\n%s", s)
	}
	if !strings.Contains(s, "# user's own comment") || !strings.Contains(s, "[mcp_servers.other]") {
		t.Fatalf("foreign content lost:\n%s", s)
	}
	twice, _, _ := MergeTOMLKey(out, "mcp_servers.gh", block)
	if string(out) != string(twice) {
		t.Fatalf("block-with-subtable merge not idempotent:\n--once--\n%s\n--twice--\n%s", out, twice)
	}
}

func TestMergeTOMLRefusesUnmanagedCollisionWithoutForce(t *testing.T) {
	if !tableExists([]byte(existingTOML), "mcp_servers.other") {
		t.Fatal("should detect existing table")
	}
	if tableExists([]byte(existingTOML), "mcp_servers.gh") {
		t.Fatal("gh table should not pre-exist")
	}
}

func TestMergeJSONKeyPreservesSiblings(t *testing.T) {
	existing := []byte(`{"theme":"dark","mcpServers":{"other":{"command":"keep"}}}`)
	managed := map[string]any{"command": "node", "args": []any{"gh.js"}}
	out, _, err := MergeJSONKey(existing, "mcpServers.gh", managed)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"theme": "dark"`) || !strings.Contains(s, `"other"`) ||
		!strings.Contains(s, `"keep"`) {
		t.Fatalf("siblings lost:\n%s", s)
	}
	if !strings.Contains(s, `"gh"`) || !strings.Contains(s, `"gh.js"`) {
		t.Fatalf("managed key not inserted:\n%s", s)
	}
}

func TestMergeJSONKeyIdempotent(t *testing.T) {
	existing := []byte(`{"mcpServers":{}}`)
	managed := map[string]any{"command": "node"}
	once, _, _ := MergeJSONKey(existing, "mcpServers.gh", managed)
	twice, _, _ := MergeJSONKey(once, "mcpServers.gh", managed)
	if string(once) != string(twice) {
		t.Fatalf("json merge not idempotent")
	}
}

// A user who owns an intermediate path segment as a non-object (string/array/number) must not
// have it silently replaced by the merge (§9.2). MergeJSONKey refuses, and jsonKeyExists flags
// it so the executor's preflight raises a collision.
func TestMergeJSONKeyRefusesNonObjectIntermediate(t *testing.T) {
	existing := []byte(`{"mcpServers":"user's own string"}`)
	if !jsonKeyExists(existing, "mcpServers.gh") {
		t.Fatal("a non-object intermediate must be reported as a collision")
	}
	if _, _, err := MergeJSONKey(existing, "mcpServers.gh", map[string]any{"command": "node"}); err == nil {
		t.Fatalf("merge must refuse to clobber a user-owned non-object intermediate")
	}
	// sanity: a genuinely-absent path is NOT a collision and merges cleanly
	if jsonKeyExists([]byte(`{"other":1}`), "mcpServers.gh") {
		t.Fatal("absent path must not be a collision")
	}
}
