// Package codex is the Codex (OpenAI) adapter target (spec §6, corrected matrix).
// Codex has NATIVE Skills at .agents/skills/<name>/SKILL.md (name+description
// required). Prompts are deprecated, so prompt/command map to a Codex skill.
// agent → emulated Codex skill. mcp → ~/.codex/config.toml [mcp_servers.<name>].
// Render is the canonical bundle-level entry point (CC-1): it iterates the bundle's
// artifacts, resolves each body via merge.Resolve (which runs fence.Strip) internally,
// and emits per-runtime output.
package codex

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/21StarkCom/bifrost/engine/internal/adapter"
	"github.com/21StarkCom/bifrost/engine/internal/adapter/emulate"
	"github.com/21StarkCom/bifrost/engine/internal/fieldmap"
	"github.com/21StarkCom/bifrost/engine/internal/merge"
	"github.com/21StarkCom/bifrost/engine/internal/model"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// version is the independently-versioned target identity (spec §7.7).
// codex@2 retargets asset references onto the Codex tree (see AssetPath /
// retargetAssets) — bodies rendered by codex@1 pointed at a Claude plugin root
// that does not exist on Codex.
const version = "codex@2"

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
	// Export STARK_ASSET_ROOT on each tool invocation so the tool's own
	// assetRoot() (STARK_ASSET_ROOT > CLAUDE_PLUGIN_ROOT > ~/.claude/code-review)
	// resolves its sibling config/prompts/standards/tools to the vendored bundle
	// root — NOT the Claude-only ~/.claude/code-review a Codex install never
	// creates. A ${VAR:-default} on the command line is substitution, not an
	// export, so the child process would otherwise never see it; the inline
	// assignment IS exported, and every sibling tool the invocation spawns inherits
	// it. This is the seam that makes vendored Codex tools actually self-contained.
	return starkToolRe.ReplaceAllStringFunc(body, func(m string) string {
		return `STARK_ASSET_ROOT="` + root + `" ` + m
	})
}

type Target struct{}

func New() *Target { return &Target{} }

func (t *Target) Runtime() model.Runtime { return model.RuntimeCodex }
func (t *Target) Version() string        { return version }

// modelMap translates canonical model ids → Codex model ids (§6.2 ActionMap).
func modelMap(canonical string) (string, bool) {
	switch modelFamily(canonical) {
	case "opus", "sonnet":
		return "gpt-5-codex", true
	case "haiku":
		return "gpt-5-mini", true
	default:
		return "", false
	}
}

func modelFamily(canonical string) string {
	s := strings.ToLower(strings.TrimSpace(canonical))
	switch {
	case s == "opus" || strings.HasPrefix(s, "opus[") || strings.HasPrefix(s, "opus-"):
		return "opus"
	case s == "sonnet" || strings.HasPrefix(s, "sonnet[") || strings.HasPrefix(s, "sonnet-"):
		return "sonnet"
	case s == "haiku" || strings.HasPrefix(s, "haiku[") || strings.HasPrefix(s, "haiku-"):
		return "haiku"
	default:
		return s
	}
}

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
		res, mf, err := merge.Resolve(a, model.RuntimeCodex)
		if err != nil {
			return nil, nil, fmt.Errorf("codex: resolve %s/%s: %w", b.Name, a.Name, err)
		}
		findings = append(findings, foldFindings(b.Name, a, mf)...)
		out, fdrops, err := t.emitArtifact(a, res)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, out...)
		findings = append(findings, fdrops...)
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

func (t *Target) emitArtifact(a *model.Artifact, res merge.Resolved) ([]adapter.OutputFile, []adapter.Finding, error) {
	switch a.Type {
	case model.TypeSkill, model.TypePrompt, model.TypeCommand:
		f, fd := t.emitSkill(a, res, false)
		return f, fd, nil
	case model.TypeAgent:
		f, fd := t.emitSkill(a, res, true) // emulated
		return f, fd, nil
	case model.TypeMCP:
		of, err := t.emitMCP(a)
		return of, nil, err
	default:
		return nil, nil, fmt.Errorf("codex: unsupported artifact type %q", a.Type)
	}
}

// emitSkill writes .agents/skills/<name>/SKILL.md. emulated=true prepends the
// §6.1 fidelity header (agents have no Codex primitive). Frontmatter is emitted via
// yaml.Marshal so values escape correctly and carried lists become YAML sequences.
func (t *Target) emitSkill(a *model.Artifact, res merge.Resolved, emulated bool) ([]adapter.OutputFile, []adapter.Finding) {
	fa := fieldmap.Apply(res.Frontmatter, a, model.RuntimeCodex, modelMap)

	desc := a.Description
	if d, ok := res.Frontmatter["description"].(string); ok && d != "" {
		desc = d
	}

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
	b.WriteString(retargetAssets(res.Body, a.Bundle))

	files := []adapter.OutputFile{{
		Path:    ".agents/skills/" + a.Name + "/SKILL.md",
		Content: []byte(fm.String() + b.String()),
	}}
	return files, dropFindings(a.Bundle, a.Name, model.RuntimeCodex, fa.Dropped)
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
	Command string            `toml:"command,omitempty"`
	Args    []string          `toml:"args,omitempty"`
	URL     string            `toml:"url,omitempty"`
	Env     map[string]string `toml:"env,omitempty"`
}

func (t *Target) emitMCP(a *model.Artifact) ([]adapter.OutputFile, error) {
	if a.MCP == nil {
		return nil, fmt.Errorf("codex: mcp artifact %q has no mcp config", a.Name)
	}
	srv := codexMCPServer{
		Command: a.MCP.Command,
		Args:    a.MCP.Args,
		URL:     a.MCP.URL,
	}
	if len(a.MCP.Env) > 0 {
		srv.Env = map[string]string{}
		// secretRef → ${KEY} placeholder; never the secret value (§4.4).
		for k := range a.MCP.Env {
			srv.Env[k] = "${" + k + "}"
		}
	}
	doc := codexMCPDoc{MCPServers: map[string]codexMCPServer{a.Name: srv}}
	out, err := toml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("codex: marshal mcp toml: %w", err)
	}
	return []adapter.OutputFile{{Path: "config.toml", Content: out}}, nil
}
