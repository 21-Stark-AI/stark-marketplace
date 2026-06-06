package install

import (
	"fmt"
	"strings"
)

// sentinel markers per spec §6.3.
func beginMarker(id string) string { return "<!-- stark:begin " + id + " -->" }
func endMarker(id string) string   { return "<!-- stark:end " + id + " -->" }

// MergeSentinel inserts/replaces the stark-managed sentinel block for `id` in a shared
// markdown file (AGENTS.md/GEMINI.md), preserving all user content outside managed blocks.
// Never blind-appends over unsentineled content. Returns (newDoc, action, error).
func MergeSentinel(doc []byte, id, body string) ([]byte, string, error) {
	s := string(doc)
	begin, end := beginMarker(id), endMarker(id)
	block := begin + "\n" + ensureTrailingNL(body) + end + "\n"

	bi := strings.Index(s, begin)
	if bi >= 0 {
		ei := strings.Index(s[bi:], end)
		if ei < 0 {
			return nil, "", fmt.Errorf("corrupt sentinel: begin %q without matching end", id)
		}
		ei = bi + ei + len(end)
		// consume a trailing newline after the end marker if present
		if ei < len(s) && s[ei] == '\n' {
			ei++
		}
		out := s[:bi] + block + s[ei:]
		return []byte(out), "replace", nil
	}
	// also reject a stray end marker without begin (corruption)
	if strings.Contains(s, end) {
		return nil, "", fmt.Errorf("corrupt sentinel: end %q without begin", id)
	}
	out := ensureTrailingNL(s)
	if out != "" && !strings.HasSuffix(out, "\n\n") {
		out += "\n"
	}
	return []byte(out + block), "insert", nil
}

// sentinelExists reports whether a managed block for id is present.
func sentinelExists(doc []byte, id string) bool {
	return strings.Contains(string(doc), beginMarker(id))
}

// removeSentinel deletes the managed block for id, preserving surrounding content.
func removeSentinel(doc []byte, id string) []byte {
	s := string(doc)
	begin, end := beginMarker(id), endMarker(id)
	bi := strings.Index(s, begin)
	if bi < 0 {
		return doc
	}
	ei := strings.Index(s[bi:], end)
	if ei < 0 {
		return doc
	}
	ei = bi + ei + len(end)
	if ei < len(s) && s[ei] == '\n' {
		ei++
	}
	// also trim one preceding blank line we may have inserted
	pre := s[:bi]
	if strings.HasSuffix(pre, "\n\n") {
		pre = pre[:len(pre)-1]
	}
	return []byte(pre + s[ei:])
}
