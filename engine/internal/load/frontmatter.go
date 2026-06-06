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
	end := bytes.Index(rest, delim)
	if end < 0 {
		return nil, "", errors.New("unterminated frontmatter: missing closing '---'")
	}
	fm = rest[:end]
	body = string(rest[end+len(delim):])
	return fm, body, nil
}
