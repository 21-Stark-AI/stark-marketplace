package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// MergeJSONKey inserts/replaces the value at a dotted key path (e.g. "mcpServers.gh") in a
// JSON object, preserving sibling keys. Re-emits with 2-space indent + sorted keys (Go's
// encoder sorts map keys) for determinism. Returns (newDoc, action, error).
func MergeJSONKey(doc []byte, dottedKey string, value any) ([]byte, string, error) {
	root := map[string]any{}
	if len(bytes.TrimSpace(doc)) > 0 {
		if err := json.Unmarshal(doc, &root); err != nil {
			return nil, "", fmt.Errorf("existing JSON invalid: %w", err)
		}
	}
	parts := strings.Split(dottedKey, ".")
	action := "insert"
	cur := root
	for i, p := range parts {
		if i == len(parts)-1 {
			if _, ok := cur[p]; ok {
				action = "replace"
			}
			cur[p] = value
			break
		}
		existing, present := cur[p]
		if !present {
			nm := map[string]any{}
			cur[p] = nm
			cur = nm
			continue
		}
		asMap, isMap := existing.(map[string]any)
		if !isMap {
			// A user owns this intermediate as a scalar/array — overwriting it would silently
			// destroy their content (§9.2). Refuse; the executor's preflight surfaces this as a
			// collision unless --force is set.
			return nil, "", fmt.Errorf("cannot merge %q: %q is occupied by a non-object value", dottedKey, p)
		}
		cur = asMap
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return append(out, '\n'), action, nil
}

// removeJSONKey deletes the dotted key path, preserving siblings; re-emits deterministically.
func removeJSONKey(doc []byte, dottedKey string) []byte {
	root := map[string]any{}
	if json.Unmarshal(doc, &root) != nil {
		return doc
	}
	parts := strings.Split(dottedKey, ".")
	cur := root
	for i, p := range parts {
		if i == len(parts)-1 {
			delete(cur, p)
			break
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			return doc
		}
		cur = next
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return doc
	}
	return append(out, '\n')
}

// jsonKeyExists reports whether the dotted key path is already present.
func jsonKeyExists(doc []byte, dottedKey string) bool {
	root := map[string]any{}
	if json.Unmarshal(doc, &root) != nil {
		return false
	}
	parts := strings.Split(dottedKey, ".")
	cur := root
	for i, p := range parts {
		if i == len(parts)-1 {
			_, ok := cur[p]
			return ok
		}
		v, present := cur[p]
		if !present {
			return false // path absent → merge will create it cleanly, no collision
		}
		next, ok := v.(map[string]any)
		if !ok {
			return true // intermediate occupied by a non-object → unmanaged collision
		}
		cur = next
	}
	return false
}
