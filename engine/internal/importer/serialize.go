package importer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
	"gopkg.in/yaml.v3"
)

// slugRE is the canonical name pattern (mirrors the validate schema). WriteBundle refuses any
// artifact/bundle name that isn't a clean slug, so a `..`/slash-bearing name can never make a
// write escape the bundle directory (defense-in-depth; validate runs only after the write).
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// serializeArtifact renders an Artifact to its canonical on-disk bytes:
// .md (frontmatter + verbatim body) for non-mcp types, plain YAML for mcp.
// Frontmatter key order is fixed (not map order) for deterministic output (spec §7.6).
func serializeArtifact(a *model.Artifact) ([]byte, error) {
	if a.Type == model.TypeMCP {
		return serializeMCP(a)
	}
	fm, err := encodeYAML(artifactFrontmatter(a))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fm)
	buf.WriteString("---\n")
	buf.WriteString(a.Body)
	return buf.Bytes(), nil
}

// artifactFrontmatter builds an ordered yaml.Node so keys serialize in canonical order.
func artifactFrontmatter(a *model.Artifact) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	put := func(k string, v any) { putKV(m, k, v) }

	put("name", a.Name)
	put("type", string(a.Type))
	put("description", a.Description)
	put("version", a.Version)
	if len(a.Tags) > 0 {
		put("tags", a.Tags)
	}
	if a.Category != "" {
		put("category", a.Category)
	}
	if a.Maturity != "" {
		put("maturity", string(a.Maturity))
	}
	if len(a.Runtimes) > 0 {
		put("runtimes", runtimeStrings(a.Runtimes))
	}
	// argument-hint is command-only in the canonical schema (artifact.command.schema.json);
	// skills/prompts/agents forbid it. Emit only for commands so the output validates clean.
	if a.ArgumentHint != "" && a.Type == model.TypeCommand {
		put("argument-hint", a.ArgumentHint)
	}
	if a.Model != "" {
		put("model", a.Model)
	}
	if a.Type == model.TypeSkill || a.Type == model.TypeCommand {
		put("disable-model-invocation", a.DisableModelInvocation)
	}
	if len(a.AllowedTools) > 0 {
		put("allowed-tools", a.AllowedTools)
	}
	return m
}

func serializeMCP(a *model.Artifact) ([]byte, error) {
	m := &yaml.Node{Kind: yaml.MappingNode}
	putKV(m, "name", a.Name)
	putKV(m, "type", string(a.Type))
	putKV(m, "description", a.Description)
	putKV(m, "version", a.Version)
	if len(a.Tags) > 0 {
		putKV(m, "tags", a.Tags)
	}
	if a.Category != "" {
		putKV(m, "category", a.Category)
	}
	// Emit maturity so the on-disk mcp file matches its IMPORT-NOTE (applyArtifactDefaults
	// defaults+notes maturity for mcp too); the mcp schema allows it.
	if a.Maturity != "" {
		putKV(m, "maturity", string(a.Maturity))
	}
	if len(a.Runtimes) > 0 {
		putKV(m, "runtimes", runtimeStrings(a.Runtimes))
	}
	putKV(m, "mcp", a.MCP)
	return encodeYAML(m)
}

func putKV(m *yaml.Node, key string, val any) {
	k := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	v := &yaml.Node{}
	if err := v.Encode(val); err != nil {
		v = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", val)}
	}
	m.Content = append(m.Content, k, v)
}

func runtimeStrings(rs []model.Runtime) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = string(r)
	}
	return out
}

func encodeYAML(node *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, err
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}

// WriteBundle writes the imported bundle under <dst>/<bundle>/ with typed subdirs
// (skills/ commands/ agents/ mcp/), bundle.yaml, and IMPORT-NOTES.md. Files are
// written with LF, 0644; dirs 0755. Atomic per-file (temp+rename).
func WriteBundle(res *ImportResult, dst string) error {
	if !slugRE.MatchString(res.Bundle.Name) {
		return fmt.Errorf("refusing to write: invalid bundle name %q (must match %s)", res.Bundle.Name, slugRE)
	}
	for _, a := range res.Bundle.Artifacts {
		if !slugRE.MatchString(a.Name) {
			return fmt.Errorf("refusing to write %s artifact: invalid name %q (must match %s)", a.Type, a.Name, slugRE)
		}
	}
	root := filepath.Join(dst, res.Bundle.Name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	by, err := serializeBundleYAML(res.Bundle)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(root, "bundle.yaml"), by); err != nil {
		return err
	}
	for _, a := range res.Bundle.Artifacts {
		rel := artifactRelPath(a)
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		data, err := serializeArtifact(a)
		if err != nil {
			return err
		}
		if err := writeFileAtomic(filepath.Join(root, rel), data); err != nil {
			return err
		}
	}
	return writeFileAtomic(filepath.Join(root, "IMPORT-NOTES.md"), renderNotes(res))
}

// artifactRelPath returns the typed subdir + filename for an artifact.
func artifactRelPath(a *model.Artifact) string {
	ext := ".md"
	if a.Type == model.TypeMCP {
		ext = ".yaml"
	}
	return typeDir(a.Type) + "/" + a.Name + ext
}

func typeDir(t model.ArtifactType) string {
	if t == model.TypeMCP {
		return "mcp"
	}
	return string(t) + "s" // skills, prompts, commands, agents
}

func serializeBundleYAML(b *model.Bundle) ([]byte, error) {
	m := &yaml.Node{Kind: yaml.MappingNode}
	putKV(m, "name", b.Name)
	putKV(m, "version", b.Version)
	putKV(m, "description", b.Description)
	if b.Category != "" {
		putKV(m, "category", b.Category)
	}
	if len(b.Tags) > 0 {
		putKV(m, "tags", b.Tags)
	}
	putKV(m, "owner", b.Owner)
	if b.Maturity != "" {
		putKV(m, "maturity", string(b.Maturity))
	}
	if len(b.Runtimes) > 0 {
		putKV(m, "runtimes", runtimeStrings(b.Runtimes))
	}
	if b.Homepage != "" {
		putKV(m, "homepage", b.Homepage)
	}
	return encodeYAML(m)
}

// renderNotes produces IMPORT-NOTES.md: a human checklist of defaulted/guessed/dropped
// fields, sorted by (where, field) for deterministic output.
func renderNotes(res *ImportResult) []byte {
	notes := append([]MetaNote(nil), res.Notes...)
	sort.Slice(notes, func(i, j int) bool {
		if notes[i].Where != notes[j].Where {
			return notes[i].Where < notes[j].Where
		}
		return notes[i].Field < notes[j].Field
	})
	var b strings.Builder
	fmt.Fprintf(&b, "# Import notes — %s\n\n", res.Bundle.Name)
	b.WriteString("Imported from stark-skills. Review and fill in each item below before merge.\n\n")
	for _, n := range notes {
		fmt.Fprintf(&b, "- [ ] **%s** · `%s` — %s\n", n.Where, n.Field, n.Note)
	}
	if len(notes) == 0 {
		b.WriteString("- (no defaulted fields — clean import)\n")
	}
	return []byte(b.String())
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
