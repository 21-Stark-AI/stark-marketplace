// Package canonjson emits deterministic JSON: sorted object keys, 2-space indent,
// no HTML escaping, LF newlines, exactly one trailing newline. It is the single
// serialization primitive behind index emission and output digests (spec §7.6).
package canonjson

import (
	"bytes"
	"encoding/json"
)

// Marshal renders v as canonical JSON. Go's encoder already sorts map[string]…
// keys; struct field order is the declared order. Disabling HTML escaping keeps
// bytes stable regardless of <, >, & in content.
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil { // Encode appends a single '\n'
		return nil, err
	}
	// Normalize any CRLF that could sneak in via string values on Windows hosts.
	return bytes.ReplaceAll(buf.Bytes(), []byte("\r\n"), []byte("\n")), nil
}
