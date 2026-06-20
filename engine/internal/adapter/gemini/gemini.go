// Package gemini is the Gemini CLI adapter target (spec §6).
// command/prompt → .gemini/commands/<name>.toml (prompt + description ONLY; args
// via {{args}}). skill/agent → emulated GEMINI.md sentinel blocks. mcp →
// settings.json mcpServers.<name>.
//
// OPEN QUESTION (spec §15.2): Gemini Extensions may be a more faithful target for
// skill/agent emulation (installable/uninstallable cleanly). This slice emits
// GEMINI.md sentinel blocks; an Extensions target can be added as gemini@2 without
// disturbing the command/mcp paths. Do not block this slice on it.
package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/emulate"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/aggregate"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/fieldmap"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/merge"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
	"github.com/pelletier/go-toml/v2"
)

const version = "gemini@1"

type Target struct{}

func New() *Target { return &Target{} }

func (t *Target) Runtime() model.Runtime { return model.RuntimeGemini }
func (t *Target) Version() string        { return version }

// Render emits Gemini output for every artifact in the bundle that targets Gemini.
// Per CC-1 it owns body resolution: merge.Resolve(a, RuntimeGemini) runs fence.Strip
// internally — the target never receives a pre-stripped body.
func (t *Target) Render(b *model.Bundle) ([]adapter.OutputFile, []adapter.Finding, error) {
	var files []adapter.OutputFile
	var findings []adapter.Finding
	for _, a := range b.Artifacts {
		if !targetsRuntime(a, model.RuntimeGemini) {
			continue
		}
		res, mf, err := merge.Resolve(a, model.RuntimeGemini)
		if err != nil {
			return nil, nil, fmt.Errorf("gemini: resolve %s/%s: %w", b.Name, a.Name, err)
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

func foldFindings(bundle string, a *model.Artifact, mf merge.Findings) []adapter.Finding {
	var out []adapter.Finding
	for _, field := range mf.ArrayDrops {
		out = append(out, adapter.Finding{
			Where: fmt.Sprintf("%s/%s@gemini", bundle, a.Name),
			Level: "warn",
			Msg:   fmt.Sprintf("override array %q drops a base prefix (likely accidental — spec §4.3)", field),
		})
	}
	if mf.Diverged {
		out = append(out, adapter.Finding{
			Where: fmt.Sprintf("%s/%s@gemini", bundle, a.Name),
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
	case model.TypeCommand, model.TypePrompt:
		return t.emitCommand(a, res)
	case model.TypeSkill, model.TypeAgent:
		f, fd := t.emitEmulated(a, res)
		return f, fd, nil
	case model.TypeMCP:
		of, err := t.emitMCP(a)
		return of, nil, err
	default:
		return nil, nil, fmt.Errorf("gemini: unsupported artifact type %q", a.Type)
	}
}

// geminiCmd is an ordered struct: only prompt + description (§6). go-toml emits
// these struct fields in declaration order → deterministic.
type geminiCmd struct {
	Description string `toml:"description"`
	Prompt      string `toml:"prompt"`
}

func (t *Target) emitCommand(a *model.Artifact, res merge.Resolved) ([]adapter.OutputFile, []adapter.Finding, error) {
	fa := fieldmap.Apply(res.Frontmatter, a, model.RuntimeGemini, nil)
	prompt := res.Body
	if hint, ok := fa.Derived["argument-hint"]; ok {
		prompt = "Usage: /" + a.Name + " " + hint + "\n\n" + res.Body
	}
	desc := a.Description
	if d, ok := res.Frontmatter["description"].(string); ok && d != "" {
		desc = d
	}
	doc := geminiCmd{Description: desc, Prompt: prompt}
	out, err := toml.Marshal(doc)
	if err != nil {
		return nil, nil, fmt.Errorf("gemini: marshal command toml: %w", err)
	}
	files := []adapter.OutputFile{{Path: ".gemini/commands/" + a.Name + ".toml", Content: out}}
	return files, dropFindings(a.Bundle, a.Name, model.RuntimeGemini, fa.Dropped), nil
}

// emitEmulated renders a skill/agent into a GEMINI.md sentinel block. The block is
// built via aggregate.Merge so the begin-sentinel digest + trailing-newline rule
// are IDENTICAL to the install-side aggregator — a single source of truth, so the
// block survives Parse→Merge round-trips (§6.3, fixes the duplicated-digest drift).
func (t *Target) emitEmulated(a *model.Artifact, res merge.Resolved) ([]adapter.OutputFile, []adapter.Finding) {
	var inner strings.Builder
	inner.WriteString(emulate.Header(a.Bundle, a.Name, "<!-- ", " -->"))
	switch a.Type {
	case model.TypeAgent:
		inner.WriteString("## Role: " + a.Name + "\n")
		inner.WriteString(a.Description + "\n\n")
	default: // skill
		inner.WriteString("## Skill: " + a.Name + "\n")
		inner.WriteString(a.Description + "\n\n")
	}
	inner.WriteString(res.Body)

	doc := aggregate.Merge([]aggregate.Section{{Bundle: a.Bundle, Name: a.Name, Content: inner.String()}})

	// surface §6.2 dropped fields (model/tools/etc. on gemini) as warnings.
	fa := fieldmap.Apply(res.Frontmatter, a, model.RuntimeGemini, nil)
	return []adapter.OutputFile{{Path: "GEMINI.md", Content: []byte(doc)}},
		dropFindings(a.Bundle, a.Name, model.RuntimeGemini, fa.Dropped)
}

// geminiMCPServer mirrors Gemini CLI settings.json mcpServers.<name>. Marshaled
// with encoding/json (object keys sorted by the standard library) for stable
// output (§7.6). One server per fragment; install merges by key (§9.2).
type geminiMCPServer struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type geminiSettings struct {
	MCPServers map[string]geminiMCPServer `json:"mcpServers"`
}

func (t *Target) emitMCP(a *model.Artifact) ([]adapter.OutputFile, error) {
	if a.MCP == nil {
		return nil, fmt.Errorf("gemini: mcp artifact %q has no mcp config", a.Name)
	}
	srv := geminiMCPServer{Command: a.MCP.Command, Args: a.MCP.Args, URL: a.MCP.URL}
	if len(a.MCP.Env) > 0 {
		srv.Env = map[string]string{}
		for k := range a.MCP.Env {
			srv.Env[k] = "${" + k + "}" // §4.4: never the secret value
		}
	}
	doc := geminiSettings{MCPServers: map[string]geminiMCPServer{a.Name: srv}}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("gemini: marshal settings.json: %w", err)
	}
	return []adapter.OutputFile{{Path: "settings.json", Content: buf.Bytes()}}, nil
}
