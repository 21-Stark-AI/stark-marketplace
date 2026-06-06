package install

import (
	"fmt"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

var tomlHeaderRE = regexp.MustCompile(`^\s*\[`)

// headerKey returns the canonical dotted key of a TOML table-header line, or "" if the line
// is not a (single, non-array) table header. Tolerant of surrounding whitespace and quoted
// key segments: `[ mcp_servers."odd name" ]` → `mcp_servers.odd name`.
func headerKey(line string) string {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "[") || strings.HasPrefix(t, "[[") || !strings.HasSuffix(t, "]") {
		return ""
	}
	inner := strings.TrimSpace(t[1 : len(t)-1])
	if inner == "" {
		return ""
	}
	parts := splitDotted(inner)
	if parts == nil {
		return ""
	}
	return strings.Join(parts, ".")
}

// splitDotted splits a dotted TOML key into segments, honoring quoted segments and stripping
// the quotes. Returns nil on malformed input.
func splitDotted(s string) []string {
	var parts []string
	var cur strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '"' || c == '\'':
			j := strings.IndexByte(s[i+1:], c)
			if j < 0 {
				return nil
			}
			cur.WriteString(s[i+1 : i+1+j])
			i = i + 1 + j + 1
			parts = append(parts, cur.String())
			cur.Reset()
			i = skipToDot(s, i)
		default:
			j := i
			for j < len(s) && s[j] != '.' && s[j] != ' ' && s[j] != '\t' {
				j++
			}
			cur.WriteString(s[i:j])
			parts = append(parts, cur.String())
			cur.Reset()
			i = skipToDot(s, j)
		}
	}
	return parts
}

// skipToDot advances past whitespace then one separating dot (if any).
func skipToDot(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
	}
	return i
}

// MergeTOMLKey inserts/replaces the managed [<dottedKey>] table in a config.toml document
// by TEXT SPLICE, preserving all foreign comments/ordering/tables. `dottedKey` is e.g.
// "mcp_servers.gh"; `managed` is the table body (lines of key=value, no header).
// Returns (newDoc, action, error). action ∈ {"insert","replace"}.
func MergeTOMLKey(doc []byte, dottedKey, managed string) ([]byte, string, error) {
	// validate the whole doc parses (don't re-emit)
	var probe map[string]any
	if len(strings.TrimSpace(string(doc))) > 0 {
		if err := toml.Unmarshal(doc, &probe); err != nil {
			return nil, "", fmt.Errorf("existing config.toml is not valid TOML: %w", err)
		}
	}
	// validate the managed body is a valid single table
	var managedProbe map[string]any
	if err := toml.Unmarshal([]byte(managed), &managedProbe); err != nil {
		return nil, "", fmt.Errorf("managed payload invalid TOML: %w", err)
	}
	// The managed payload may be either a flat body (key=value lines, no header — the shape
	// the test fakes emit) OR a complete block that already carries its own [dottedKey] header
	// plus child subtables like [dottedKey.env] (the shape a real runtime target renders, since
	// go-toml emits nested maps as subtables). Any header that is NOT dottedKey and NOT a
	// dottedKey.* subtable is foreign and rejected — that keeps the "no smuggling a sibling
	// table" guard while letting the artifact carry its own structure.
	prefix := dottedKey + "."
	hasOwnHeader := false
	for _, ln := range strings.Split(managed, "\n") {
		hk := headerKey(ln)
		if hk == "" {
			if tomlHeaderRE.MatchString(ln) {
				return nil, "", fmt.Errorf("managed payload has unsupported header: %q", strings.TrimSpace(ln))
			}
			continue
		}
		switch {
		case hk == dottedKey:
			hasOwnHeader = true
		case strings.HasPrefix(hk, prefix):
			// own subtable — allowed
		default:
			return nil, "", fmt.Errorf("managed payload must only define [%s] (+subtables); found foreign header %q", dottedKey, strings.TrimSpace(ln))
		}
	}
	var managedBlock string
	if hasOwnHeader {
		managedBlock = ensureTrailingNL(managed) // already a complete [dottedKey] block
	} else {
		managedBlock = "[" + dottedKey + "]\n" + ensureTrailingNL(managed)
	}

	lines := strings.Split(string(doc), "\n")
	start, end := findTable(lines, dottedKey)
	if start < 0 {
		out := ensureTrailingNL(string(doc))
		if out != "" && !strings.HasSuffix(out, "\n\n") {
			out += "\n"
		}
		return []byte(out + managedBlock), "insert", nil
	}
	// replace [start,end)
	newLines := append([]string{}, lines[:start]...)
	newLines = append(newLines, strings.Split(strings.TrimRight(managedBlock, "\n"), "\n")...)
	newLines = append(newLines, lines[end:]...)
	// ensureTrailingNL keeps replace convergent with insert: when the managed table sat at
	// EOF, splicing out its trailing blank line would otherwise drop the final newline,
	// making a second merge differ from the first (idempotency break).
	return []byte(ensureTrailingNL(strings.Join(newLines, "\n"))), "replace", nil
}

// findTable returns [start,end) line indices covering the [<dottedKey>] table, tolerant of
// whitespace/quoted header keys. The table runs until the next header that is NOT a subtable
// of dottedKey (so [<dottedKey>.sub] stays part of the managed region), or EOF.
func findTable(lines []string, dottedKey string) (start, end int) {
	start = -1
	for i, ln := range lines {
		if headerKey(ln) == dottedKey {
			start = i
			break
		}
	}
	if start < 0 {
		return -1, -1
	}
	prefix := dottedKey + "."
	for j := start + 1; j < len(lines); j++ {
		hk := headerKey(lines[j])
		if hk == "" {
			continue // comment / kv / blank — part of the table body
		}
		if hk == dottedKey || strings.HasPrefix(hk, prefix) {
			continue // a subtable of the managed key — keep consuming
		}
		return start, j
	}
	return start, len(lines)
}

// tableExists reports whether [<dottedKey>] is present (used for unmanaged-collision check).
func tableExists(doc []byte, dottedKey string) bool {
	s, _ := findTable(strings.Split(string(doc), "\n"), dottedKey)
	return s >= 0
}

// ExtractTOMLTable returns the complete [dottedKey] table block (its header + body + own
// subtables) from a TOML document, or "" if the table is absent. The install adapter uses this
// to pull one artifact's managed table out of a rendered config fragment that go-toml/v2 may
// prefix with a bare parent header (e.g. a `[mcp_servers]` line before `[mcp_servers.gh]`) —
// only the keyed table is the managed payload.
func ExtractTOMLTable(doc []byte, dottedKey string) string {
	lines := strings.Split(string(doc), "\n")
	start, end := findTable(lines, dottedKey)
	if start < 0 {
		return ""
	}
	return ensureTrailingNL(strings.Join(lines[start:end], "\n"))
}

func ensureTrailingNL(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

// removeTOMLTable deletes the [<dottedKey>] table (and its subtables) by text splice,
// leaving foreign content untouched.
func removeTOMLTable(doc []byte, dottedKey string) []byte {
	lines := strings.Split(string(doc), "\n")
	start, end := findTable(lines, dottedKey)
	if start < 0 {
		return doc
	}
	out := append([]string{}, lines[:start]...)
	out = append(out, lines[end:]...)
	return []byte(strings.Join(out, "\n"))
}
