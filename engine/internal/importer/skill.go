package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
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
		case strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`),
			strings.HasPrefix(v, `'`) && strings.HasSuffix(v, `'`):
			return m // already quoted
		}
		q, err := json.Marshal(v) // valid YAML double-quoted scalar (JSON string syntax)
		if err != nil {
			return m
		}
		return append(append([]byte{}, prefix...), q...)
	})
}

// decodeFrontmatter parses frontmatter as strict YAML, falling back to a sanitized re-parse
// (quoting loose argument-hint values) for real stark-skills files. Returns whether the
// fallback was needed so the caller can record it for human review.
func decodeFrontmatter(fm []byte) (raw map[string]any, sanitized bool, err error) {
	if err = yaml.Unmarshal(fm, &raw); err == nil {
		return raw, false, nil
	}
	if err2 := yaml.Unmarshal(sanitizeFrontmatter(fm), &raw); err2 != nil {
		return nil, false, err // surface the ORIGINAL error (more informative)
	}
	return raw, true, nil
}

// importSkills walks <from>/skill/<name>/SKILL.md and maps each to a model.Artifact.
// Missing skill/ dir is not an error (a plugin-only import is valid).
func importSkills(from, bundle string, res *ImportResult) error {
	skillRoot := filepath.Join(from, "skill")
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
	where := bundle + "/skill/" + a.Name
	if sanitized {
		res.note(where, "frontmatter", "source frontmatter required sanitizing (loose unquoted value) — verify mapped fields")
	}
	noteDroppedSourceFields(raw, res, where)
	// argument-hint is command-only canonically; a skill that carried one in stark-skills loses
	// it on import — surface that so the human can fold it into the description if it matters.
	if _, ok := raw["argument-hint"]; ok {
		res.note(where, "argument-hint", "argument-hint is command-only; dropped from this skill — move any usage hint into the description")
	}
	applyArtifactDefaults(a, res, where)
	return a, nil
}

// mapCommonFrontmatter copies the carryable canonical fields from a raw frontmatter map.
// Shared by skills and commands (both use the same key shapes in stark-skills).
func mapCommonFrontmatter(a *model.Artifact, raw map[string]any) {
	if v, ok := raw["name"].(string); ok {
		a.Name = v
	}
	if v, ok := raw["description"].(string); ok {
		a.Description = strings.TrimSpace(v)
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

// noteDroppedSourceFields records source-only frontmatter keys (revision/revision_date)
// that have no canonical equivalent and are intentionally dropped.
func noteDroppedSourceFields(raw map[string]any, res *ImportResult, where string) {
	for _, k := range sourceOnlyFields {
		if _, ok := raw[k]; ok {
			res.note(where, k, "source-only field dropped (no canonical equivalent)")
		}
	}
}
