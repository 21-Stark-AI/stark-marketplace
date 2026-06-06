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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/adapter"
	"github.com/GetEvinced/stark-marketplace/engine/internal/adapter/emulate"
	"github.com/GetEvinced/stark-marketplace/engine/internal/fieldmap"
	"github.com/GetEvinced/stark-marketplace/engine/internal/merge"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
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
		out, err := t.emitArtifact(a, res.Body)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, out...)
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

func (t *Target) emitArtifact(a *model.Artifact, body string) ([]adapter.OutputFile, error) {
	switch a.Type {
	case model.TypeCommand, model.TypePrompt:
		return t.emitCommand(a, body)
	case model.TypeSkill, model.TypeAgent:
		return t.emitEmulated(a, body), nil
	case model.TypeMCP:
		return t.emitMCP(a)
	default:
		return nil, fmt.Errorf("gemini: unsupported artifact type %q", a.Type)
	}
}

// geminiCmd is an ordered struct: only prompt + description (§6). go-toml emits
// these struct fields in declaration order → deterministic.
type geminiCmd struct {
	Description string `toml:"description"`
	Prompt      string `toml:"prompt"`
}

func (t *Target) emitCommand(a *model.Artifact, body string) ([]adapter.OutputFile, error) {
	res := fieldmap.Apply(a, model.RuntimeGemini, nil)
	prompt := body
	if hint, ok := res.Derived["argument-hint"]; ok {
		prompt = "Usage: /" + a.Name + " " + hint + "\n\n" + body
	}
	doc := geminiCmd{Description: a.Description, Prompt: prompt}
	out, err := toml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal command toml: %w", err)
	}
	return []adapter.OutputFile{{
		Path:    ".gemini/commands/" + a.Name + ".toml",
		Content: out,
	}}, nil
}

// sectionDigest is a short content digest used in the begin sentinel so install
// can detect drift. Pure function of the rendered inner content.
func sectionDigest(inner string) string {
	sum := sha256.Sum256([]byte(inner))
	return hex.EncodeToString(sum[:])[:12]
}

func (t *Target) emitEmulated(a *model.Artifact, body string) []adapter.OutputFile {
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
	inner.WriteString(body)

	id := a.Bundle + "/" + a.Name
	innerStr := inner.String()
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- stark:begin %s@%s -->\n", id, sectionDigest(innerStr))
	b.WriteString(innerStr)
	if !strings.HasSuffix(innerStr, "\n") {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "<!-- stark:end %s -->\n", id)

	return []adapter.OutputFile{{Path: "GEMINI.md", Content: []byte(b.String())}}
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
