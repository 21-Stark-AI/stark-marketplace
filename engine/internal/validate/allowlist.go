package validate

// commandAllowlist is the positive set of MCP command basenames (spec §7.4).
// Governance for additions is tracked in spec §15.4. Keep minimal.
var commandAllowlist = map[string]bool{
	"node": true, "npx": true, "uvx": true,
	"stark-bq-mcp": true, "stark-gh-mcp": true,
}

// inlineEvalFlags are forbidden in MCP args (arbitrary code on start).
var inlineEvalFlags = map[string]bool{
	"-e": true, "-c": true, "--eval": true, "-p": true, "--print": true,
}
