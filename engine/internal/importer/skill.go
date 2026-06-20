package importer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
	"gopkg.in/yaml.v3"
)

// argHintLine matches a top-level `argument-hint:` frontmatter line and captures its prefix +
// raw value. stark-skills authors write this free-form hint unquoted, often starting with a
// YAML-significant char (`[patch|minor|major] (optional)`), which strict YAML rejects.
var argHintLine = regexp.MustCompile(`(?m)^(argument-hint:[ \t]*)(\S.*?)[ \t]*$`)

// sanitizeFrontmatter quotes the free-form argument-hint value so loose-but-real stark-skills
// frontmatter parses as strict YAML. Already-quoted or block-scalar values are left untouched.
func sanitizeFrontmatter(fm []byte) []byte {
	return argHintLine.ReplaceAllFunc(fm, func(m []byte) []byte {
		sub := argHintLine.FindSubmatch(m)
		prefix, v := sub[1], strings.TrimSpace(string(sub[2]))
		switch {
		case v == "", v == ">", v == ">-", v == ">+", v == "|", v == "|-", v == "|+":
			return m // block-scalar indicator — leave it to YAML
		case strings.HasPrefix(v, `"`), strings.HasPrefix(v, `'`):
			// already a quoted scalar — leave it to YAML (which also handles a trailing
			// `# comment`; matching only the prefix avoids re-quoting `"[a|b]"  # pick one`).
			return m
		}
		q, err := json.Marshal(v) // valid YAML double-quoted scalar (JSON string syntax)
		if err != nil {
			return m
		}
		return append(append([]byte{}, prefix...), q...)
	})
}

// decodeFrontmatter parses frontmatter, ALWAYS quoting a loose argument-hint first. This is
// run unconditionally (not only on a parse error) because the common real shape
// `argument-hint: [start|end]` is VALID YAML — it parses as a flow sequence ([]any), not a
// string, and would otherwise be silently dropped by the .(string) carry. Quoting it up front
// makes it a string literal; `sanitized` reports whether anything was rewritten so the caller
// can flag it for review.
func decodeFrontmatter(fm []byte) (raw map[string]any, sanitized bool, err error) {
	clean := sanitizeFrontmatter(fm)
	sanitized = !bytes.Equal(clean, fm)
	if err = yaml.Unmarshal(clean, &raw); err != nil {
		// sanitizing only quotes argument-hint; if the cleaned form somehow fails, retry the
		// original so we surface the most informative parse error.
		if err2 := yaml.Unmarshal(fm, &raw); err2 != nil {
			return nil, false, err
		}
		return raw, false, nil
	}
	return raw, sanitized, nil
}

// importSkills maps <from>/skill/<name>/SKILL.md files into the bundle. When `only` is
// non-empty, exactly those skills are imported in the given order and a missing/typo'd name
// is a hard error (fail-closed — you get the bundle you asked for). When `only` is empty,
// every skill under skill/ is imported in sorted order. A missing skill/ dir is not an error
// (a plugin-only import is valid).
func importSkills(from, bundle string, only []string, res *ImportResult) error {
	skillRoot := filepath.Join(from, "skill")

	if len(only) > 0 {
		seen := make(map[string]bool, len(only))
		for _, name := range only {
			if seen[name] {
				return fmt.Errorf("skill %q: duplicated in the requested set", name)
			}
			seen[name] = true
			path := filepath.Join(skillRoot, name, "SKILL.md")
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("skill %q: not found under %s (%w)", name, skillRoot, err)
			}
			a, err := mapSkillFile(path, bundle, res)
			if err != nil {
				return fmt.Errorf("skill %s: %w", name, err)
			}
			res.Bundle.Artifacts = append(res.Bundle.Artifacts, a)
		}
		return nil
	}

	entries, err := os.ReadDir(skillRoot)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // deterministic order
	for _, name := range names {
		path := filepath.Join(skillRoot, name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			continue // dir without a SKILL.md (e.g. evals/) is skipped
		}
		a, err := mapSkillFile(path, bundle, res)
		if err != nil {
			return fmt.Errorf("skill %s: %w", name, err)
		}
		res.Bundle.Artifacts = append(res.Bundle.Artifacts, a)
	}
	return nil
}

// mapSkillFile reads one SKILL.md, maps known frontmatter to the canonical superset,
// preserves the body verbatim, and records defaulted/dropped fields.
func mapSkillFile(path, bundle string, res *ImportResult) (*model.Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fm, body, err := splitFrontmatter(normalizeLF(data))
	if err != nil {
		return nil, err
	}
	raw, sanitized, err := decodeFrontmatter(fm)
	if err != nil {
		return nil, err
	}
	a := &model.Artifact{
		Type:   model.TypeSkill,
		Bundle: bundle,
		Body:   cleanBody(body),
	}
	mapCommonFrontmatter(a, raw)
	// stark-skills derive identity from the directory; fall back to it when `name:` is absent or
	// non-string, so the output is never a name-less artifact that fails validate.
	nameDerived := false
	if a.Name == "" {
		a.Name = filepath.Base(filepath.Dir(path))
		nameDerived = true
	}
	where := bundle + "/skill/" + a.Name
	if nameDerived {
		res.note(where, "name", "name derived from the source directory (frontmatter had none) — confirm")
	}
	if sanitized {
		res.note(where, "argument-hint", "argument-hint was reformatted from a loose/unquoted source value — verify it")
	}
	noteUnmappedFields(raw, res, where)
	// argument-hint is command-only canonically; a skill that carried one in stark-skills loses
	// it on import — surface that so the human can fold it into the description if it matters.
	if _, ok := raw["argument-hint"]; ok {
		res.note(where, "argument-hint", "argument-hint is command-only; dropped from this skill — move any usage hint into the description")
	}
	applyArtifactDefaults(a, res, where)
	return a, nil
}

// mapCommonFrontmatter copies the carryable canonical fields from a raw frontmatter map.
// Shared by skills and commands. It carries EVERY schema-valid field the source provides —
// including version/tags/category/maturity/summary/runtimes — so a source value is never
// silently discarded and then misreported by applyArtifactDefaults as "defaulted".
func mapCommonFrontmatter(a *model.Artifact, raw map[string]any) {
	if v, ok := raw["name"].(string); ok {
		a.Name = v
	}
	if v, ok := raw["description"].(string); ok {
		a.Description = strings.TrimSpace(v)
	}
	if v, ok := raw["version"].(string); ok {
		a.Version = v
	}
	if v, ok := raw["category"].(string); ok {
		a.Category = v
	}
	if v, ok := raw["summary"].(string); ok {
		a.Summary = v
	}
	if v, ok := raw["maturity"].(string); ok {
		a.Maturity = model.Maturity(v)
	}
	if tags := parseToolList(raw["tags"]); len(tags) > 0 {
		a.Tags = tags
	}
	if rts := parseRuntimes(raw["runtimes"]); len(rts) > 0 {
		a.Runtimes = rts
	}
	if v, ok := raw["argument-hint"].(string); ok {
		a.ArgumentHint = v
	}
	if v, ok := raw["model"].(string); ok {
		a.Model = v
	}
	if v, ok := raw["disable-model-invocation"].(bool); ok {
		a.DisableModelInvocation = v
	}
	a.AllowedTools = parseToolList(raw["allowed-tools"])
}

func parseRuntimes(v any) []model.Runtime {
	var out []model.Runtime
	for _, s := range parseToolList(v) {
		out = append(out, model.Runtime(s))
	}
	return out
}

// handledKeys are the frontmatter keys mapCommonFrontmatter carries (or that the type-specific
// note logic explicitly handles). Anything else is reported by noteUnmappedFields.
var handledKeys = map[string]bool{
	"name": true, "type": true, "description": true, "version": true, "category": true,
	"summary": true, "maturity": true, "tags": true, "runtimes": true,
	"argument-hint": true, "model": true, "disable-model-invocation": true, "allowed-tools": true,
}

// parseToolList accepts either a YAML list or a comma-separated string ("Bash, Read").
func parseToolList(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case string:
		var out []string
		for _, s := range strings.Split(t, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// noteUnmappedFields records EVERY source frontmatter key the importer did not carry/handle
// (spec §12: surface every dropped field). This is a residual sweep, not a fixed allowlist, so
// real keys like `context: fork` or `revision` are reported instead of silently lost.
func noteUnmappedFields(raw map[string]any, res *ImportResult, where string) {
	var keys []string
	for k := range raw {
		if !handledKeys[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		res.note(where, k, "source field dropped — no canonical equivalent; fold into the description/overrides if it carries meaning")
	}
}
