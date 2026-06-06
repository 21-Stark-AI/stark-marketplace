package validate

import "testing"

func TestSchemaRejectsStringEnv(t *testing.T) {
	doc := map[string]any{
		"name": "bigquery", "type": "mcp", "description": "x", "version": "1.0.0",
		"mcp": map[string]any{
			"transport": "stdio", "command": "stark-bq-mcp",
			"env": map[string]any{"BQ": "literal-secret"}, // illegal string
		},
	}
	if err := ValidateSchema("mcp", doc); err == nil {
		t.Fatal("expected schema error for string env value")
	}
}

func TestSchemaAcceptsSecretRef(t *testing.T) {
	doc := map[string]any{
		"name": "bigquery", "type": "mcp", "description": "x", "version": "1.0.0",
		"mcp": map[string]any{
			"transport": "stdio", "command": "stark-bq-mcp",
			"env": map[string]any{"BQ": map[string]any{"secretRef": "bq-id"}},
		},
	}
	if err := ValidateSchema("mcp", doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
