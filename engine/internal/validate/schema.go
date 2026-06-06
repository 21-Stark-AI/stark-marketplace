package validate

import (
	"bytes"
	"embed"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed all:schema
var schemaFS embed.FS // embeds engine/internal/validate/schema/*.json (authored in Task 4)

var (
	compileOnce sync.Once
	compiled    map[string]*jsonschema.Schema
	compileErr  error
)

// The `requires` subschema is inlined into each artifact schema (no shared
// "stark:requires" resource), so compile() only loads the per-type schemas.
func compile() {
	c := jsonschema.NewCompiler()
	compiled = map[string]*jsonschema.Schema{}
	for _, t := range []string{"skill", "command", "prompt", "agent", "mcp"} {
		data, err := schemaFS.ReadFile("schema/artifact." + t + ".schema.json")
		if err != nil {
			compileErr = err
			return
		}
		doc, err := jsonschema.UnmarshalJSON(bytesReader(data))
		if err != nil {
			compileErr = err
			return
		}
		id := "stark:artifact." + t
		if err := c.AddResource(id, doc); err != nil {
			compileErr = err
			return
		}
		s, err := c.Compile(id)
		if err != nil {
			compileErr = err
			return
		}
		compiled[t] = s
	}
}

// ValidateSchema validates a JSON-faithful frontmatter value (see Artifact.Raw —
// produced via a YAML→JSON round-trip) against the schema for its type.
func ValidateSchema(typ string, doc any) error {
	compileOnce.Do(compile)
	if compileErr != nil {
		return fmt.Errorf("schema compile: %w", compileErr)
	}
	s, ok := compiled[typ]
	if !ok {
		return fmt.Errorf("no schema for type %q", typ)
	}
	return s.Validate(doc)
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
