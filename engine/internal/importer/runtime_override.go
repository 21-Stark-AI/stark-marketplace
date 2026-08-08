package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/21StarkCom/bifrost/engine/internal/model"
)

const codexOverrideReason = "source-owned Codex runtime override"

// runtimeOverrideFields is the canonical frontmatter surface that a source
// runtime overlay may replace. Source-only metadata such as revision and
// revision_date remains intentionally unmapped, just as it does for the base
// stark-skills import.
var runtimeOverrideFields = [...]string{
	"name",
	"type",
	"description",
	"version",
	"tags",
	"category",
	"maturity",
	"summary",
	"runtimes",
	"requires",
	"argument-hint",
	"model",
	"disable-model-invocation",
	"allowed-tools",
	"tools",
}

func attachCodexSkillOverride(from string, a *model.Artifact) error {
	path := filepath.Join(from, "runtime-overrides", "codex", "skill", a.Name, "SKILL.md")
	return attachCodexMarkdownOverride(from, path, a)
}

func attachCodexCommandOverride(from, bundle, filename string, a *model.Artifact) error {
	path := filepath.Join(from, "runtime-overrides", "codex", "plugins", bundle, "commands", filename)
	return attachCodexMarkdownOverride(from, path, a)
}

// attachCodexMarkdownOverride loads an optional source-owned overlay. The base
// artifact remains the Claude-compatible source; only merge.Resolve(...,
// RuntimeCodex) sees this full frontmatter/body replacement.
// An artifact whose declared runtimes exclude Codex (e.g. `runtimes: [claude]`
// in SKILL.md frontmatter) is exempt from the overlay requirement — the
// adapters already skip it at render time, so demanding a Codex variant here
// would force authoring one for a skill that deliberately ships Claude-only.
// Runtimes are always populated at this point: mapSkillFile/mapCommandFile
// apply the [claude, codex] default before the attach runs.
func attachCodexMarkdownOverride(from, path string, a *model.Artifact) error {
	if !artifactTargetsCodex(a) {
		return nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		root := filepath.Join(from, "runtime-overrides", "codex")
		if fi, rootErr := os.Stat(root); os.IsNotExist(rootErr) {
			return nil // legacy/fixture source with no overlay feature enabled
		} else if rootErr != nil {
			return fmt.Errorf("inspect Codex runtime override root %s: %w", root, rootErr)
		} else if !fi.IsDir() {
			return fmt.Errorf("Codex runtime override root %s is not a directory", root)
		}
		return fmt.Errorf("Codex runtime override is enabled but artifact %q is missing: %s", a.Name, path)
	}
	if err != nil {
		return fmt.Errorf("read Codex runtime override %s: %w", path, err)
	}

	fm, body, err := splitFrontmatter(normalizeLF(data))
	if err != nil {
		return fmt.Errorf("Codex runtime override %s: %w", path, err)
	}
	raw, _, err := decodeFrontmatter(fm)
	if err != nil {
		return fmt.Errorf("Codex runtime override %s: %w", path, err)
	}

	name, ok := raw["name"].(string)
	if !ok || name == "" {
		return fmt.Errorf("Codex runtime override %s: name is required", path)
	}
	if name != a.Name {
		return fmt.Errorf("Codex runtime override %s: name %q != canonical artifact %q", path, name, a.Name)
	}
	if typ, ok := raw["type"]; ok && fmt.Sprint(typ) != string(a.Type) {
		return fmt.Errorf("Codex runtime override %s: type %q != canonical artifact type %q", path, typ, a.Type)
	}
	if description, ok := raw["description"].(string); !ok || strings.TrimSpace(description) == "" {
		return fmt.Errorf("Codex runtime override %s: description is required", path)
	}

	fields := make(map[string]any)
	for _, key := range runtimeOverrideFields {
		value, present := raw[key]
		if !present {
			continue
		}
		normalized, err := normalizeRuntimeOverrideField(key, value)
		if err != nil {
			return fmt.Errorf("Codex runtime override %s: %w", path, err)
		}
		fields[key] = normalized
	}

	if a.Overrides == nil {
		a.Overrides = make(map[model.Runtime]model.Override)
	}
	if _, exists := a.Overrides[model.RuntimeCodex]; exists {
		return fmt.Errorf("Codex runtime override %s: artifact %q already has a Codex override", path, a.Name)
	}
	a.Overrides[model.RuntimeCodex] = model.Override{
		Fields: fields,
		Body:   "# diverged: " + codexOverrideReason + "\n" + cleanBody(body),
	}
	return nil
}

func artifactTargetsCodex(a *model.Artifact) bool {
	for _, r := range a.Runtimes {
		if r == model.RuntimeCodex {
			return true
		}
	}
	return false
}

func normalizeRuntimeOverrideField(key string, value any) (any, error) {
	switch key {
	case "name", "type", "description", "version", "category", "maturity", "summary", "argument-hint", "model":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("field %q must be a string", key)
		}
		if key == "description" {
			s = strings.TrimSpace(s)
		}
		return s, nil
	case "disable-model-invocation":
		b, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("field %q must be a boolean", key)
		}
		return b, nil
	case "tags", "runtimes", "allowed-tools", "tools":
		items := parseToolList(value)
		if items == nil {
			return nil, fmt.Errorf("field %q must be a string list", key)
		}
		return items, nil
	case "requires":
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported runtime override field %q", key)
	}
}
