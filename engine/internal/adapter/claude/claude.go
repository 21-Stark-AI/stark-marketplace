// Package claude is the native Claude Code adapter target (spec §6 matrix):
// skills/<name>/SKILL.md, commands/<name>.md, agents/<name>.md, .mcp.json.
// Output is deterministic: ordered frontmatter fields, LF, `/` separators,
// sorted map keys in JSON. The target is independently versioned (spec §7.7).
package claude

import (
	"fmt"
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/adapter"
	"github.com/GetEvinced/stark-marketplace/engine/internal/canonjson"
	"github.com/GetEvinced/stark-marketplace/engine/internal/merge"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
	"gopkg.in/yaml.v3"
)

// Version is the Claude adapter target version (spec §7.7). Bump on any format
// change; it participates in the output/provenance digest.
const Version = "claude@1"

// emitFrontmatter renders the given fields, in the given order, as a `---` block.
// Empty string / nil / empty-slice values are skipped. Scalar values are emitted
// via yaml.Marshal so quoting/escaping matches a real YAML reader exactly.
func emitFrontmatter(order []string, fm map[string]any) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, k := range order {
		v, ok := fm[k]
		if !ok || isEmpty(v) {
			continue
		}
		out, err := yaml.Marshal(map[string]any{k: v})
		if err != nil {
			out = []byte(fmt.Sprintf("%s: %v\n", k, v))
		}
		b.Write(out)
	}
	b.WriteString("---\n")
	return b.String()
}

func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []string:
		return len(t) == 0
	case []any:
		return len(t) == 0
	case bool:
		// Booleans are NEVER "empty": a boolean PRESENT in the resolved frontmatter
		// is emitted explicitly, including `false` (F-Cov#9). Absence is controlled
		// upstream by selectFields, which only inserts the key when the
		// artifact/override actually carries it.
		_ = t
		return false
	default:
		return false
	}
}

// Target is the Claude adapter target.
type Target struct{}

// New returns a Claude target.
func New() *Target { return &Target{} }

func (*Target) Runtime() model.Runtime { return model.RuntimeClaude }
func (*Target) Version() string        { return Version }

// field orders per output type (spec §6 frontmatter columns).
var (
	skillOrder   = []string{"name", "description", "disable-model-invocation", "allowed-tools", "model"}
	commandOrder = []string{"name", "description", "argument-hint", "model", "allowed-tools"}
	agentOrder   = []string{"name", "description", "tools", "model"}
)

// Render emits native Claude files for every claude-targeted artifact in b.
// It returns a flat []adapter.Finding per CC-1. The divergence budget is
// recomputed by the build orchestrator from the "diverged" findings.
func (t *Target) Render(b *model.Bundle) ([]adapter.OutputFile, []adapter.Finding, error) {
	var files []adapter.OutputFile
	var findings []adapter.Finding
	for _, a := range b.Artifacts {
		if !targets(a, model.RuntimeClaude) {
			continue
		}
		res, mf, err := merge.Resolve(a, model.RuntimeClaude)
		if err != nil {
			return nil, findings, err
		}
		findings = append(findings, foldFindings(b.Name, a, mf)...)

		switch a.Type {
		case model.TypeSkill:
			fm := selectFields(res.Frontmatter, a)
			files = append(files, adapter.OutputFile{
				Path:    "skills/" + a.Name + "/SKILL.md",
				Content: []byte(emitFrontmatter(skillOrder, fm) + res.Body),
			})
		case model.TypeCommand, model.TypePrompt:
			fm := selectFields(res.Frontmatter, a)
			files = append(files, adapter.OutputFile{
				Path:    "commands/" + a.Name + ".md",
				Content: []byte(emitFrontmatter(commandOrder, fm) + res.Body),
			})
		case model.TypeAgent:
			fm := selectFields(res.Frontmatter, a)
			files = append(files, adapter.OutputFile{
				Path:    "agents/" + a.Name + ".md",
				Content: []byte(emitFrontmatter(agentOrder, fm) + res.Body),
			})
		case model.TypeMCP:
			// handled by renderMCP (.mcp.json aggregation)
		}
	}
	if mf := t.renderMCP(b); mf != nil {
		files = append(files, *mf)
	}
	if pj := renderPluginJSON(b); pj != nil {
		files = append(files, *pj)
	}
	adapter.SortFiles(files)
	return files, findings, nil
}

// renderPluginJSON emits `.claude-plugin/plugin.json` so each rendered bundle is
// a self-contained, installable Claude Code plugin (testable directly via
// `claude --plugin-dir dist/claude/<bundle>`), not only resolvable through the
// repo-root marketplace manifest. Fields mirror the manifest entry; author comes
// from the bundle owner.
func renderPluginJSON(b *model.Bundle) *adapter.OutputFile {
	pj := map[string]any{
		"name":        b.Name,
		"description": b.Description,
	}
	if b.Version != "" {
		pj["version"] = b.Version
	}
	if b.Owner.Name != "" {
		author := map[string]any{"name": b.Owner.Name}
		if b.Owner.Email != "" {
			author["email"] = b.Owner.Email
		}
		pj["author"] = author
	}
	if len(b.Tags) > 0 {
		pj["keywords"] = b.Tags
	}
	if b.Homepage != "" {
		pj["homepage"] = b.Homepage
	}
	content, _ := canonjson.Marshal(pj)
	return &adapter.OutputFile{Path: ".claude-plugin/plugin.json", Content: content}
}

// selectFields builds the frontmatter map from resolved frontmatter, falling
// back to the typed artifact fields so output is well-defined even when Raw is
// empty (e.g. inherited values). Resolved overrides win.
func selectFields(res map[string]any, a *model.Artifact) map[string]any {
	fm := map[string]any{
		"name":          a.Name,
		"description":   a.Description,
		"argument-hint": a.ArgumentHint,
		"model":         a.Model,
	}
	if a.DisableModelInvocation {
		fm["disable-model-invocation"] = true
	}
	if len(a.AllowedTools) > 0 {
		fm["allowed-tools"] = a.AllowedTools
	}
	if len(a.Tools) > 0 {
		fm["tools"] = a.Tools
	}
	// resolved overrides take precedence over typed defaults. This also lets an
	// authored boolean that is explicitly `false` in the source frontmatter (e.g.
	// `disable-model-invocation: false`) survive to output (F-Cov#9): it lands in
	// res via merge.Resolve and is copied here even though the typed-default block
	// above only inserts the key when true.
	for _, k := range []string{"name", "description", "argument-hint", "model", "disable-model-invocation", "allowed-tools", "tools"} {
		if v, ok := res[k]; ok {
			fm[k] = v
		}
	}
	return fm
}

func targets(a *model.Artifact, rt model.Runtime) bool {
	for _, r := range a.Runtimes {
		if r == rt {
			return true
		}
	}
	return false
}

// foldFindings translates merge.Findings into the canonical flat []adapter.Finding
// (CC-1). Divergence is surfaced as a "warn"-level finding whose Msg begins with
// "diverged:" so the build orchestrator can count it for the budget.
func foldFindings(bundle string, a *model.Artifact, mf merge.Findings) []adapter.Finding {
	var out []adapter.Finding
	for _, field := range mf.ArrayDrops {
		out = append(out, adapter.Finding{
			Where: fmt.Sprintf("%s/%s", bundle, a.Name),
			Level: "warn",
			Msg:   fmt.Sprintf("override array %q drops a base prefix (likely accidental — spec §4.3)", field),
		})
	}
	if mf.Diverged {
		out = append(out, adapter.Finding{
			Where: fmt.Sprintf("%s/%s@claude", bundle, a.Name),
			Level: "warn",
			Msg:   "diverged: " + mf.DivergedReason,
		})
	}
	return out
}

// rewriteMCPPath prefixes a bundle-relative server-script path with
// ${CLAUDE_PLUGIN_ROOT} so it resolves once the plugin is installed (plugins are
// copied to an opaque cache dir, so bare/relative paths would not resolve). Bare
// executables (node, npx), absolute paths, flags (-x), `~`, and paths already
// rooted at a shell variable are left untouched.
func rewriteMCPPath(s string) string {
	if s == "" ||
		strings.HasPrefix(s, "-") ||
		strings.HasPrefix(s, "/") ||
		strings.HasPrefix(s, "$") ||
		strings.HasPrefix(s, "~") {
		return s
	}
	// Only rewrite things that look like an in-bundle script: a relative path
	// (contains a slash) or a bare filename with a known script extension.
	if strings.Contains(s, "/") || hasScriptExt(s) {
		return "${CLAUDE_PLUGIN_ROOT}/" + strings.TrimPrefix(s, "./")
	}
	return s
}

func hasScriptExt(s string) bool {
	for _, ext := range []string{".js", ".mjs", ".cjs", ".ts", ".py", ".sh"} {
		if strings.HasSuffix(s, ext) {
			return true
		}
	}
	return false
}

// renderMCP aggregates all claude-targeted MCP artifacts into one `.mcp.json`.
// Returns nil when the bundle has no MCP artifacts. Server map keys are sorted
// by canonjson; env preserves the secretRef object form (spec §4.4) for install.
func (*Target) renderMCP(b *model.Bundle) *adapter.OutputFile {
	servers := map[string]any{}
	for _, a := range b.Artifacts {
		if a.Type != model.TypeMCP || a.MCP == nil || !targets(a, model.RuntimeClaude) {
			continue
		}
		entry := map[string]any{"transport": a.MCP.Transport}
		if a.MCP.Command != "" {
			entry["command"] = rewriteMCPPath(a.MCP.Command)
		}
		if len(a.MCP.Args) > 0 {
			args := make([]string, len(a.MCP.Args))
			for i, arg := range a.MCP.Args {
				args[i] = rewriteMCPPath(arg)
			}
			entry["args"] = args
		}
		if a.MCP.URL != "" {
			entry["url"] = a.MCP.URL
		}
		if len(a.MCP.Env) > 0 {
			env := map[string]any{}
			for k, v := range a.MCP.Env {
				env[k] = map[string]any{"secretRef": v.SecretRef}
			}
			entry["env"] = env
		}
		servers[a.Name] = entry
	}
	if len(servers) == 0 {
		return nil
	}
	content, _ := canonjson.Marshal(map[string]any{"mcpServers": servers})
	return &adapter.OutputFile{Path: ".mcp.json", Content: content}
}
