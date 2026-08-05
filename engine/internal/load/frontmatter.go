package load

import (
	"bytes"
	"errors"
)

var delim = []byte("---\n")

// splitFrontmatter splits a `---\n…\n---\n` YAML header from the markdown body.
// Input is assumed normalized to LF (the loader enforces this on read).
func splitFrontmatter(data []byte) (fm []byte, body string, err error) {
	if !bytes.HasPrefix(data, delim) {
		return nil, "", errors.New("missing frontmatter: file must start with '---'")
	}
	rest := data[len(delim):]
	end := frontmatterEnd(rest)
	if end < 0 {
		return nil, "", errors.New("unterminated frontmatter: missing closing '---'")
	}
	fm = rest[:end]
	body = string(rest[end+len(delim):])
	return fm, body, nil
}

// frontmatterEnd returns the offset of a closing delimiter at column zero.
// Searching for delim directly is not sufficient: YAML block scalars may contain
// indented Markdown rules ("    ---"), which are frontmatter content rather than
// delimiters.
func frontmatterEnd(rest []byte) int {
	if bytes.HasPrefix(rest, delim) { // empty frontmatter
		return 0
	}
	if i := bytes.Index(rest, []byte("\n---\n")); i >= 0 {
		return i + 1
	}
	return -1
}
