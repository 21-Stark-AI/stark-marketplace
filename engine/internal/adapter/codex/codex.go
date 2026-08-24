// Package codex is the Codex (OpenAI) adapter target (spec §6, corrected matrix).
// Codex has NATIVE Skills at .agents/skills/<name>/SKILL.md (name+description
// required). Prompts are deprecated, so prompt/command map to a Codex skill.
// agent → emulated Codex skill. mcp → .codex/config.toml [mcp_servers.<name>].
// Render is the canonical bundle-level entry point (CC-1): it iterates the bundle's
// artifacts, resolves each body via merge.Resolve (which runs fence.Strip) internally,
// and emits per-runtime output.
package codex

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/21StarkCom/bifrost/engine/internal/adapter"
	"github.com/21StarkCom/bifrost/engine/internal/adapter/emulate"
	"github.com/21StarkCom/bifrost/engine/internal/canonjson"
	"github.com/21StarkCom/bifrost/engine/internal/fieldmap"
	"github.com/21StarkCom/bifrost/engine/internal/merge"
	"github.com/21StarkCom/bifrost/engine/internal/model"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// version is the independently-versioned target identity (spec §7.7).
// codex@2 retargeted asset references onto the Codex tree. codex@3 emits native
// agents/openai.yaml invocation policy and stops writing the unsupported model
// field into SKILL.md. codex@4 adds native-plugin .mcp.json aggregation.
const version = "codex@4"

// AssetsRoot is the Codex-tree directory holding ONE bundle's vendored
// stark-skills assets (tools/, standards/, prompts/, scripts/, config.json).
// It is the Codex analogue of dist/claude/<bundle>/ for a Claude plugin: a root
// per bundle, never a shared one — stark-gh ships its own config.json, which
// would clobber the shared snapshot's in a flat namespace.
func AssetsRoot(bundle string) string { return ".agents/stark/" + bundle }

// AssetPath maps a vendored asset's bundle-relative path (as it sits in
// dist/claude/<bundle>/) to its path in the Codex tree.
//
// Per-skill assets (skills/<name>/references/**) must land NEXT TO the skill
// that references them, and Codex skills live at .agents/skills/<name>/ — so
// those keep their shape under .agents/. Everything else is bundle-scoped and
// goes to the bundle's assets root.
func AssetPath(bundle, rel string) string {
	if strings.HasPrefix(rel, "skills/") {
		return ".agents/" + rel
	}
	return AssetsRoot(bundle) + "/" + rel
}

// pluginRootSkillsRe matches ${CLAUDE_PLUGIN_ROOT...}/skills/ — a body reference
// into a PER-SKILL asset (skills/<name>/references/**). AssetPath routes those to
// .agents/skills/<name>/ (next to the skill), NOT the bundle's own root, so the
// blanket pluginRootRe rewrite below would send them to the wrong place. This runs
// first and re-points them via the bundle root's siblings: on both a global
// (~/.agents/stark/<bundle>) and a project-local (<dest>/.agents/stark/<bundle>)
// install, <root>/../../skills IS .agents/skills, so one rule covers both modes.
var pluginRootSkillsRe = regexp.MustCompile(`\$\{CLAUDE_PLUGIN_ROOT(?::-[^}]*)?\}/skills/`)

// pluginRootRe matches ${CLAUDE_PLUGIN_ROOT} and its ${CLAUDE_PLUGIN_ROOT:-...}
// defaulted form. The variable is injected by Claude Code and is never set on
// Codex, so both forms would resolve to the Claude fallback (or to empty).
var pluginRootRe = regexp.MustCompile(`\$\{CLAUDE_PLUGIN_ROOT(?::-[^}]*)?\}`)

// relAssetRe matches a SKILL.md-relative reference into a vendored asset dir
// (../../standards/help.md and friends). On Claude a skill sits at
// <plugin>/skills/<name>/SKILL.md so ../../ IS the plugin root, while a command
// sits at <plugin>/commands/<name>.md so its natural ref is a single ../; on Codex
// BOTH classes emit at .agents/skills/<name>/SKILL.md (emitSkill), so any leading
// ../-run is one or more levels short of the bundle's assets root. Match a run of
// one-or-more ../ and re-point the whole thing at the bundle root.
var relAssetRe = regexp.MustCompile(`(?:\.\./)+(tools|standards|prompts|scripts)/`)

// starkToolRe matches the stark tool runner signature `node --experimental-strip-types`.
// Every vendored tool is a .ts invoked this way; inline `node -e '...'` snippets do NOT
// carry the flag, so they are correctly left alone.
var starkToolRe = regexp.MustCompile(`node\s+--experimental-strip-types`)

// starkPluginRootRe matches both the plain and defaulted forms of the portable
// Stark root. A native plugin must not fall back to the standalone
// $HOME/.agents/stark tree because marketplace packages live in Codex's plugin
// cache instead.
var starkPluginRootRe = regexp.MustCompile(`\$\{STARK_PLUGIN_ROOT(?::-[^{}]*)?\}`)

// These two forms occur in portable source overlays. Match the complete nested
// expression before replacing its CLAUDE_PLUGIN_ROOT leaf; otherwise a simple
// inner replacement would create a malformed nested STARK_PLUGIN_ROOT value.
var assetPluginClaudeRootRe = regexp.MustCompile(`\$\{STARK_ASSET_ROOT:-\$\{STARK_PLUGIN_ROOT:-\$\{CLAUDE_PLUGIN_ROOT(?::-[^}]*)?\}\}\}`)
var pluginClaudeRootRe = regexp.MustCompile(`\$\{STARK_PLUGIN_ROOT:-\$\{CLAUDE_PLUGIN_ROOT(?::-[^}]*)?\}\}`)

const pluginAssetPreamble = `## Codex plugin asset root

For every shell invocation that reads this skill's packaged files, first
resolve the absolute directory containing this loaded ` + "`SKILL.md`" + ` from the skill
path Codex supplied. In that same shell invocation set ` + "`SKILL_DIR`" + ` to that
directory, set ` + "`STARK_PLUGIN_ROOT`" + ` to the absolute ` + "`../..`" + ` directory, and
export it. In every such shell invocation also set and export
` + "`STARK_STATE_ROOT=\"${STARK_STATE_ROOT:-$HOME/.stark/code-review}\"`" + `. Do not derive
the plugin root from the current working directory, do not reuse a value from
an earlier shell invocation, and do not write Codex state under ` + "`~/.claude`" + `.

`

const codexStateRootAssignment = `STARK_STATE_ROOT="${STARK_STATE_ROOT:-$HOME/.stark/code-review}"`

func retargetToolInvocations(body, root string) string {
	return starkToolRe.ReplaceAllStringFunc(body, func(m string) string {
		return `env STARK_ASSET_ROOT="` + root + `" ` + codexStateRootAssignment + ` ` + m
	})
}

// retargetAssets rewrites a SKILL.md body's asset references onto the Codex tree
// layout that BundleAssets installs. Without it every rendered skill points at
// paths that exist only inside a Claude plugin.
func retargetAssets(body, bundle string) string {
	// Depth-independent rewrites (var refs + tool-invocation env) — safe for any
	// vendored text file, shared with BundleAssets via RetargetPluginRefs.
	body = RetargetPluginRefs(body, bundle)
	// SKILL.md-depth-specific: relative ../ asset refs. Only valid for bodies
	// emitted at .agents/skills/<name>/SKILL.md, so NOT applied to vendored files.
	return relAssetRe.ReplaceAllString(body, "../../stark/"+bundle+"/$1/")
}

// RetargetPluginRefs rewrites the DEPTH-INDEPENDENT asset references — the
// ${CLAUDE_PLUGIN_ROOT} variable and stark-tool invocations — onto the Codex tree.
// It is exported so BundleAssets can apply it to vendored prose (references,
// standards, prompts) that ship these refs verbatim; the relative-path rewrite is
// deliberately excluded there because a vendored file's on-disk depth differs from
// a SKILL.md's, so ../ math cannot be assumed.
func RetargetPluginRefs(body, bundle string) string {
	root := "${STARK_PLUGIN_ROOT:-$HOME/" + AssetsRoot(bundle) + "}"
	// Per-skill asset refs first (see pluginRootSkillsRe) — before the blanket rule
	// consumes the variable. ReplaceAllLiteralString: the replacement holds $HOME.
	body = pluginRootSkillsRe.ReplaceAllLiteralString(body, root+"/../../skills/")
	// Blanket: every remaining ${CLAUDE_PLUGIN_ROOT...} → the bundle's Codex root.
	body = pluginRootRe.ReplaceAllLiteralString(body, root)
	// Forward STARK_ASSET_ROOT and STARK_STATE_ROOT through env on each tool
	// invocation so shell commands and argv arrays share one valid command shape.
	// The Codex tool overlay's own assetRoot()
	// (STARK_ASSET_ROOT > STARK_PLUGIN_ROOT > ~/.stark/code-review)
	// resolves its sibling config/prompts/standards/tools to the vendored bundle
	// root — NOT the Claude-only ~/.claude/code-review a Codex install never
	// creates. env exports both assignments to the child, and every sibling tool
	// the invocation spawns inherits them. This is the seam that makes vendored
	// Codex tools self-contained without relying on shell assignment position.
	return retargetToolInvocations(body, root)
}

// retargetPluginSkill rewrites a resolved skill body for a native plugin root.
// Unlike the standalone layout, a plugin keeps assets directly beside skills:
//
//	<plugin>/skills/<name>/SKILL.md
//	<plugin>/{tools,standards,...}
//
// Codex exposes the loaded skill path to the model, but does not promise an
// ambient PLUGIN_ROOT variable for arbitrary shell commands launched from skill
// prose. The preamble therefore makes the root resolution explicit and every
// rewritten reference fails closed on that invocation-scoped root.
func retargetPluginSkill(body string) string {
	root := "${STARK_PLUGIN_ROOT:?resolve from this loaded SKILL.md as instructed above}"
	body = assetPluginClaudeRootRe.ReplaceAllLiteralString(body, "${STARK_ASSET_ROOT:-"+root+"}")
	body = pluginClaudeRootRe.ReplaceAllLiteralString(body, root)
	body = pluginRootSkillsRe.ReplaceAllLiteralString(body, root+"/skills/")
	body = pluginRootRe.ReplaceAllLiteralString(body, root)
	body = starkPluginRootRe.ReplaceAllLiteralString(body, root)
	body = retargetToolInvocations(body, root)
	return pluginAssetPreamble + body
}

// RetargetPluginAssetRefs applies the depth-independent portion of the native
// plugin rewrite to prose assets such as references and standards. Relative
// paths are intentionally untouched because each asset has its own depth.
func RetargetPluginAssetRefs(body string) string {
	root := "${STARK_PLUGIN_ROOT:?resolve from the loaded SKILL.md as instructed}"
	body = assetPluginClaudeRootRe.ReplaceAllLiteralString(body, "${STARK_ASSET_ROOT:-"+root+"}")
	body = pluginClaudeRootRe.ReplaceAllLiteralString(body, root)
	body = pluginRootSkillsRe.ReplaceAllLiteralString(body, root+"/skills/")
	body = pluginRootRe.ReplaceAllLiteralString(body, root)
	body = starkPluginRootRe.ReplaceAllLiteralString(body, root)
	return retargetToolInvocations(body, root)
}

type layout uint8

const (
	layoutStandalone layout = iota
	layoutPlugin
)

// Target renders either the standalone Codex install tree or a native Codex
// plugin package. The standalone layout is the long-standing `stark install`
// contract; plugin layout is deliberately separate because its skill and asset
// roots are different.
type Target struct {
	layout layout
}

func New() *Target { return &Target{layout: layoutStandalone} }

// NewPlugin renders a native Codex plugin root. Skills are direct children of
// ./skills and asset references resolve against the plugin root. It is used by
// the committed Codex marketplace build only; New remains byte-compatible for
// direct installs.
func NewPlugin() *Target { return &Target{layout: layoutPlugin} }

func (t *Target) Runtime() model.Runtime { return model.RuntimeCodex }
func (t *Target) Version() string        { return version }

// Render emits Codex output for every artifact in the bundle that targets Codex.
// Per CC-1 it owns body resolution: merge.Resolve(a, RuntimeCodex) runs fence.Strip
// internally — the target never receives a pre-stripped body. merge.Resolve returns
// (Resolved, Findings, error); the resolved frontmatter+body drive emission and
// merge findings + dropped-field warnings are folded into the flat []adapter.Finding.
func (t *Target) Render(b *model.Bundle) ([]adapter.OutputFile, []adapter.Finding, error) {
	var files []adapter.OutputFile
	var findings []adapter.Finding
	for _, a := range b.Artifacts {
		if !targetsRuntime(a, model.RuntimeCodex) {
			continue
		}
		if t.layout == layoutPlugin && a.Type == model.TypeMCP {
			continue // aggregated once at the plugin root below
		}
		res, mf, err := merge.Resolve(a, model.RuntimeCodex)
		if err != nil {
			return nil, nil, fmt.Errorf("codex: resolve %s/%s: %w", b.Name, a.Name, err)
		}
		findings = append(findings, foldFindings(b.Name, a, mf)...)
		out, fdrops, err := t.emitArtifact(b, a, res)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, out...)
		findings = append(findings, fdrops...)
	}
	if t.layout == layoutPlugin {
		mcpFile, err := t.renderPluginMCP(b)
		if err != nil {
			return nil, nil, err
		}
		if mcpFile != nil {
			files = append(files, *mcpFile)
		}
	}
	return files, findings, nil
}

func targetsRuntime(a *model.Artifact, rt model.Runtime) bool {
	for _, r := range a.Runtimes {
		if r == rt {
			return true
		}
	}
	return false
}

// foldFindings translates merge.Findings into the canonical flat []adapter.Finding
// (CC-1): array foot-guns and author-divergence surface as warn-level findings.
func foldFindings(bundle string, a *model.Artifact, mf merge.Findings) []adapter.Finding {
	var out []adapter.Finding
	for _, field := range mf.ArrayDrops {
		out = append(out, adapter.Finding{
			Where: fmt.Sprintf("%s/%s@codex", bundle, a.Name),
			Level: "warn",
			Msg:   fmt.Sprintf("override array %q drops a base prefix (likely accidental — spec §4.3)", field),
		})
	}
	if mf.Diverged {
		out = append(out, adapter.Finding{
			Where: fmt.Sprintf("%s/%s@codex", bundle, a.Name),
			Level: "warn",
			Msg:   "diverged: " + mf.DivergedReason,
		})
	}
	return out
}

// dropFindings surfaces §6.2 drop+warn fields as warn-level findings.
func dropFindings(bundle, name string, rt model.Runtime, dropped []string) []adapter.Finding {
	var out []adapter.Finding
	for _, f := range dropped {
		out = append(out, adapter.Finding{
			Where: fmt.Sprintf("%s/%s@%s", bundle, name, rt),
			Level: "warn",
			Msg:   fmt.Sprintf("field %q dropped on %s (§6.2)", f, rt),
		})
	}
	return out
}

func (t *Target) emitArtifact(bundle *model.Bundle, a *model.Artifact, res merge.Resolved) ([]adapter.OutputFile, []adapter.Finding, error) {
	switch a.Type {
	case model.TypeSkill, model.TypePrompt, model.TypeCommand:
		f, fd := t.emitSkill(bundle, a, res, false)
		return f, fd, nil
	case model.TypeAgent:
		f, fd := t.emitSkill(bundle, a, res, true) // emulated
		return f, fd, nil
	case model.TypeMCP:
		if t.layout == layoutPlugin {
			return nil, nil, fmt.Errorf("codex plugin: MCP artifact %q requires plugin-root .mcp.json aggregation", a.Name)
		}
		of, err := t.emitMCP(a)
		return of, nil, err
	default:
		return nil, nil, fmt.Errorf("codex: unsupported artifact type %q", a.Type)
	}
}

// emitSkill writes .agents/skills/<name>/SKILL.md. emulated=true prepends the
// §6.1 fidelity header (agents have no Codex primitive). Frontmatter is emitted via
// yaml.Marshal so values escape correctly and carried lists become YAML sequences.
func (t *Target) emitSkill(bundle *model.Bundle, a *model.Artifact, res merge.Resolved, emulated bool) ([]adapter.OutputFile, []adapter.Finding) {
	fa := fieldmap.Apply(res.Frontmatter, a, model.RuntimeCodex, nil)

	desc := a.Description
	if d, ok := res.Frontmatter["description"].(string); ok && d != "" {
		desc = d
	}
	desc = TranslateInvocationReferences(desc, bundle)

	var fm strings.Builder
	fm.WriteString("---\n")
	// name + description are REQUIRED by Codex skills.
	writeYAMLField(&fm, "name", a.Name)
	writeYAMLField(&fm, "description", desc)
	// carried fields, sorted by key for determinism (§7.6).
	for _, k := range sortedAnyKeys(fa.Carried) {
		writeYAMLField(&fm, k, fa.Carried[k])
	}
	fm.WriteString("---\n")

	var b strings.Builder
	if emulated {
		b.WriteString(emulate.Header(a.Bundle, a.Name, "<!-- ", " -->"))
	}
	// derived fields render as usage prose (§6.2: argument-hint → usage note).
	if hint, ok := fa.Derived["argument-hint"]; ok {
		b.WriteString("Usage: " + a.Name + " " + hint + "\n\n")
	}
	root := ".agents/skills/"
	body := retargetAssets(res.Body, a.Bundle)
	if t.layout == layoutPlugin {
		root = "skills/"
		body = retargetPluginSkill(res.Body)
	}
	b.WriteString(body)

	files := []adapter.OutputFile{{
		Path:    root + a.Name + "/SKILL.md",
		Content: []byte(fm.String() + b.String()),
	}}

	// Claude commands are explicit slash-command surfaces, not auto-selected
	// workflows. Preserve that boundary when a command is represented as a Codex
	// skill. Native source skills carry the same intent through
	// disable-model-invocation, mapped by fieldmap into this product metadata.
	disableModelInvocation, hasDisableModelInvocation := fa.Derived["disable-model-invocation"]
	hasInvocationPolicy := a.Type == model.TypeCommand || hasDisableModelInvocation
	// A command remains explicit-only even though the canonical serializer writes
	// `disable-model-invocation: false` for commands by default. That false value
	// describes the source frontmatter; it must not weaken the stricter semantic
	// boundary introduced by representing a slash command as a Codex skill.
	allowImplicit := a.Type != model.TypeCommand && disableModelInvocation != "true"
	if hasInvocationPolicy {
		files = append(files, adapter.OutputFile{
			Path:    root + a.Name + "/agents/openai.yaml",
			Content: renderOpenAIYAML(a.Name, desc, allowImplicit),
		})
	}
	return files, dropFindings(a.Bundle, a.Name, model.RuntimeCodex, fa.Dropped)
}

// TranslateInvocationReferences projects authored Claude slash-command prose
// onto Codex's dollar-prefixed skill invocation syntax. Only names of artifacts
// that actually target Codex are translated, so paths and references to
// unavailable Claude-only commands remain untouched.
func TranslateInvocationReferences(description string, b *model.Bundle) string {
	if b == nil || description == "" {
		return description
	}

	type invocationReference struct {
		authored string
		codex    string
	}
	references := make([]invocationReference, 0, len(b.Artifacts)*2)
	seen := make(map[string]struct{}, len(b.Artifacts)*2)
	addReference := func(authored, codex string) {
		if authored == "" || codex == "" {
			return
		}
		if _, ok := seen[authored]; ok {
			return
		}
		seen[authored] = struct{}{}
		references = append(references, invocationReference{authored: authored, codex: codex})
	}
	for _, a := range b.Artifacts {
		if a == nil || a.Name == "" || !targetsRuntime(a, model.RuntimeCodex) {
			continue
		}
		addReference(a.Name, a.Name)
		// Claude plugin commands may be invoked through their bundle-qualified
		// namespace. Native Codex exposes each command artifact as a flat skill.
		// Keep this mapping type-aware: skills and agents do not acquire a
		// command-only bundle alias.
		if a.Type == model.TypeCommand && b.Name != "" {
			addReference(b.Name+":"+a.Name, a.Name)
		}
	}
	// Prefer the longest authored form when one is a prefix of another.
	sort.Slice(references, func(i, j int) bool {
		if len(references[i].authored) != len(references[j].authored) {
			return len(references[i].authored) > len(references[j].authored)
		}
		return references[i].authored < references[j].authored
	})

	var out strings.Builder
	out.Grow(len(description))
	for i := 0; i < len(description); {
		var matched *invocationReference
		if description[i] == '/' && invocationBoundaryBefore(description, i) {
			for j := range references {
				ref := &references[j]
				end := i + 1 + len(ref.authored)
				if end <= len(description) && description[i+1:end] == ref.authored && invocationBoundaryAfter(description, end) {
					matched = ref
					break
				}
			}
		}
		if matched != nil {
			out.WriteByte('$')
			out.WriteString(matched.codex)
			i += 1 + len(matched.authored)
			continue
		}
		out.WriteByte(description[i])
		i++
	}
	return out.String()
}

func invocationBoundaryBefore(s string, slash int) bool {
	if slash == 0 {
		return true
	}
	return !invocationNameByte(s[slash-1]) && s[slash-1] != '/' && s[slash-1] != '$' && s[slash-1] != '.'
}

func invocationBoundaryAfter(s string, end int) bool {
	return end == len(s) || !invocationNameByte(s[end]) && s[end] != '/'
}

func invocationNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-' || b == '_'
}

// renderOpenAIYAML emits the Codex-native home for invocation policy. The
// interface keys are required whenever agents/openai.yaml exists; derive compact,
// deterministic values from the canonical identity rather than inventing a
// second authored metadata source.
func renderOpenAIYAML(name, description string, allowImplicit bool) []byte {
	display := displayName(name)
	short := shortDescription(description)
	return []byte(fmt.Sprintf(
		"interface:\n  display_name: %s\n  short_description: %s\npolicy:\n  allow_implicit_invocation: %t\n",
		quoteYAMLString(display), quoteYAMLString(short), allowImplicit,
	))
}

func quoteYAMLString(s string) string {
	b, _ := json.Marshal(s) // every Go string is JSON/YAML representable
	return string(b)
}

func displayName(name string) string {
	acronyms := map[string]string{
		"adr": "ADR", "cc": "CC", "gh": "GH", "gha": "GHA",
		"ssot": "SSOT", "iac": "IaC", "pr": "PR",
	}
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if v, ok := acronyms[part]; ok {
			parts[i] = v
			continue
		}
		r := []rune(part)
		if len(r) > 0 {
			r[0] = unicode.ToUpper(r[0])
		}
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}

// shortDescription follows the Codex UI guidance (25-64 characters). Collapse
// authored wrapping, then cut at the last word boundary so the result remains a
// readable blurb rather than a byte-truncated fragment.
func shortDescription(description string) string {
	s := strings.Join(strings.Fields(description), " ")
	if len([]rune(s)) < 25 {
		stem := strings.TrimSpace(strings.TrimRight(s, ".!?"))
		if stem == "" {
			stem = "Guided skill"
		}
		s = stem + " workflow instructions for Codex."
	}
	r := []rune(s)
	if len(r) <= 64 {
		return s
	}
	cut := 64
	for cut > 25 && !unicode.IsSpace(r[cut]) {
		cut--
	}
	return strings.TrimSpace(string(r[:cut]))
}

// writeYAMLField appends `k: <yaml-encoded v>` using yaml.Marshal so scalars are
// quoted/escaped when needed and slices emit as YAML sequences — valid, deterministic.
func writeYAMLField(b *strings.Builder, k string, v any) {
	out, err := yaml.Marshal(map[string]any{k: v})
	if err != nil {
		fmt.Fprintf(b, "%s: %v\n", k, v)
		return
	}
	b.Write(out)
}

func sortedAnyKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// codexMCPDoc is an ordered struct (NOT a map) so go-toml emits deterministic
// key order (§7.6). One server per emitted fragment; install merges by key (§9.2).
type codexMCPDoc struct {
	MCPServers map[string]codexMCPServer `toml:"mcp_servers"`
}

type codexMCPServer struct {
	Command string   `toml:"command,omitempty"`
	Args    []string `toml:"args,omitempty"`
	URL     string   `toml:"url,omitempty"`
	EnvVars []string `toml:"env_vars,omitempty"`
}

func validateMCP(a *model.Artifact) error {
	if a.MCP == nil {
		return fmt.Errorf("codex: mcp artifact %q has no mcp config", a.Name)
	}
	switch a.MCP.Transport {
	case "stdio":
		if a.MCP.Command == "" {
			return fmt.Errorf("codex: stdio mcp artifact %q requires command", a.Name)
		}
		if a.MCP.URL != "" {
			return fmt.Errorf("codex: stdio mcp artifact %q cannot set url", a.Name)
		}
	case "http":
		if a.MCP.URL == "" {
			return fmt.Errorf("codex: http mcp artifact %q requires url", a.Name)
		}
		if a.MCP.Command != "" || len(a.MCP.Args) > 0 {
			return fmt.Errorf("codex: http mcp artifact %q cannot set command or args", a.Name)
		}
		if len(a.MCP.Env) > 0 {
			return fmt.Errorf("codex: http mcp artifact %q cannot map canonical env to HTTP authentication; model bearer_token_env_var or env_http_headers explicitly", a.Name)
		}
	default:
		return fmt.Errorf("codex: mcp artifact %q has unsupported transport %q", a.Name, a.MCP.Transport)
	}
	return nil
}

func (t *Target) emitMCP(a *model.Artifact) ([]adapter.OutputFile, error) {
	if err := validateMCP(a); err != nil {
		return nil, err
	}
	srv := codexMCPServer{
		Command: a.MCP.Command,
		Args:    a.MCP.Args,
		URL:     a.MCP.URL,
	}
	if a.MCP.Transport == "stdio" && len(a.MCP.Env) > 0 {
		// Codex forwards named variables from the launch environment with
		// `env_vars`. Its `env` table contains literal values and does not expand
		// `${KEY}` placeholders, so writing secret-looking strings there silently
		// disconnects the server from the actual credential.
		for k := range a.MCP.Env {
			srv.EnvVars = append(srv.EnvVars, k)
		}
		sort.Strings(srv.EnvVars)
	}
	doc := codexMCPDoc{MCPServers: map[string]codexMCPServer{a.Name: srv}}
	out, err := toml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("codex: marshal mcp toml: %w", err)
	}
	return []adapter.OutputFile{{Path: ".codex/config.toml", Content: out}}, nil
}

// renderPluginMCP aggregates Codex-targeted MCP artifacts into the native
// plugin-root .mcp.json contract. Canonical secretRef mappings are rejected for
// native plugins until the format has a non-literal forwarding primitive; this
// prevents a generated package from embedding credentials or inert placeholders.
func (t *Target) renderPluginMCP(b *model.Bundle) (*adapter.OutputFile, error) {
	servers := map[string]any{}
	for _, a := range b.Artifacts {
		if a.Type != model.TypeMCP || !targetsRuntime(a, model.RuntimeCodex) {
			continue
		}
		if err := validateMCP(a); err != nil {
			return nil, err
		}
		if len(a.MCP.Env) > 0 {
			return nil, fmt.Errorf("codex plugin: mcp artifact %q cannot safely encode secretRef env in .mcp.json", a.Name)
		}
		entry := map[string]any{}
		if a.MCP.Command != "" {
			entry["command"] = a.MCP.Command
		}
		if len(a.MCP.Args) > 0 {
			entry["args"] = append([]string(nil), a.MCP.Args...)
		}
		if a.MCP.URL != "" {
			entry["url"] = a.MCP.URL
		}
		servers[a.Name] = entry
	}
	if len(servers) == 0 {
		return nil, nil
	}
	content, err := canonjson.Marshal(servers)
	if err != nil {
		return nil, fmt.Errorf("codex plugin: marshal .mcp.json: %w", err)
	}
	return &adapter.OutputFile{Path: ".mcp.json", Content: content}, nil
}
