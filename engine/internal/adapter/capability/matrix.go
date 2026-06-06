// Package capability is the versioned source of truth for per-(type,runtime)
// support levels (spec §6). It is plain data — surfaced in the index for
// native/emulated/unsupported badges and consumed by validation (§7.4).
package capability

import "github.com/GetEvinced/stark-marketplace/engine/internal/model"

// Version bumps whenever any cell changes; it is independent of adapter target
// versions (§7.7) and lets the index communicate matrix revisions.
const Version = "1"

type key struct {
	t  model.ArtifactType
	rt model.Runtime
}

// matrix encodes the corrected §6 capability matrix (red-team Part B).
var matrix = map[key]model.SupportLevel{
	// ── Claude Code: everything native ──
	{model.TypeSkill, model.RuntimeClaude}:   model.SupportNative,
	{model.TypePrompt, model.RuntimeClaude}:  model.SupportNative,
	{model.TypeCommand, model.RuntimeClaude}: model.SupportNative,
	{model.TypeAgent, model.RuntimeClaude}:   model.SupportNative,
	{model.TypeMCP, model.RuntimeClaude}:     model.SupportNative,

	// ── Codex (OpenAI): native Skills at .agents/skills/<name>/SKILL.md ──
	// prompts deprecated → command/prompt map to a Codex skill (still native shape).
	{model.TypeSkill, model.RuntimeCodex}:   model.SupportNative,
	{model.TypePrompt, model.RuntimeCodex}:  model.SupportNative,
	{model.TypeCommand, model.RuntimeCodex}: model.SupportNative,
	{model.TypeAgent, model.RuntimeCodex}:   model.SupportEmulated, // no subagent primitive
	{model.TypeMCP, model.RuntimeCodex}:     model.SupportNative,   // ~/.codex/config.toml [mcp_servers.<name>]

	// ── Gemini CLI ──
	{model.TypePrompt, model.RuntimeGemini}:  model.SupportNative, // .gemini/commands/<name>.toml
	{model.TypeCommand, model.RuntimeGemini}: model.SupportNative,
	{model.TypeSkill, model.RuntimeGemini}:   model.SupportEmulated, // GEMINI.md sentinel block
	{model.TypeAgent, model.RuntimeGemini}:   model.SupportEmulated, // GEMINI.md role block
	{model.TypeMCP, model.RuntimeGemini}:     model.SupportNative,   // settings.json mcpServers.<name>
}

// Level returns the support level for a (type, runtime) pair. Unknown pairs are
// treated as unsupported (fail-closed).
func Level(t model.ArtifactType, rt model.Runtime) model.SupportLevel {
	if lvl, ok := matrix[key{t, rt}]; ok {
		return lvl
	}
	return model.SupportUnsupported
}
