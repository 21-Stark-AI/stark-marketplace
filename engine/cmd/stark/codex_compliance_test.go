package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/21StarkCom/bifrost/engine/internal/indexio"
	"github.com/21StarkCom/bifrost/engine/internal/load"
	"github.com/21StarkCom/bifrost/engine/internal/model"
	"gopkg.in/yaml.v3"
)

var skillSupportRefRe = regexp.MustCompile(`(?:references|scripts|assets)/[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*`)

func splitSkillFrontmatter(t *testing.T, content string) (map[string]any, string) {
	t.Helper()
	if !strings.HasPrefix(content, "---\n") {
		t.Fatal("SKILL.md does not start with YAML frontmatter")
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		t.Fatal("SKILL.md has no closing frontmatter fence")
	}
	end += 4
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(content[4:end]), &fm); err != nil {
		t.Fatalf("invalid SKILL.md frontmatter: %v", err)
	}
	return fm, content[end+5:]
}

type openAIMetadata struct {
	Interface struct {
		DisplayName      string `yaml:"display_name"`
		ShortDescription string `yaml:"short_description"`
	} `yaml:"interface"`
	Policy struct {
		AllowImplicitInvocation *bool `yaml:"allow_implicit_invocation"`
	} `yaml:"policy"`
}

// TestEveryCommittedCodexSkillMeetsNativeContract installs every committed
// bundle through the production adapter, then validates the actual files Codex
// discovers. This is intentionally inventory-derived: adding a skill without
// extending a hand-maintained test list cannot create a false green.
func TestEveryCommittedCodexSkillMeetsNativeContract(t *testing.T) {
	root := repoRoot(t)
	idx, err := indexio.LoadIndex(filepath.Join(root, "index.json"))
	if err != nil {
		t.Skipf("committed index.json not present (%v)", err)
	}

	expected := map[string]bool{}
	explicitOnly := map[string]bool{}
	bundleSet := map[string]bool{}
	for _, a := range idx.Artifacts {
		support, ok := a.Support[model.RuntimeCodex]
		if !ok || support == model.SupportUnsupported || a.Type == model.TypeMCP {
			continue
		}
		expected[a.Name] = true
		bundleSet[a.Bundle] = true
	}
	cat, err := load.Load(filepath.Join(root, "catalog"))
	if err != nil {
		t.Fatal(err)
	}
	for _, bundle := range cat.Bundles {
		for _, artifact := range bundle.Artifacts {
			if !expected[artifact.Name] {
				continue
			}
			explicitOnly[artifact.Name] = artifact.Type == model.TypeCommand || artifact.DisableModelInvocation
		}
	}
	bundles := make([]string, 0, len(bundleSet))
	for bundle := range bundleSet {
		bundles = append(bundles, bundle)
	}
	sort.Strings(bundles)

	allowedFrontmatter := map[string]bool{
		"name": true, "description": true, "license": true,
		"metadata": true, "allowed-tools": true,
	}
	seen := map[string]bool{}
	for _, bundle := range bundles {
		dest := liveCodexInstall(t, bundle)
		paths, err := filepath.Glob(filepath.Join(dest, ".agents", "skills", "*", "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, skillPath := range paths {
			name := filepath.Base(filepath.Dir(skillPath))
			if seen[name] {
				continue
			}
			seen[name] = true
			t.Run(name, func(t *testing.T) {
				data, err := os.ReadFile(skillPath)
				if err != nil {
					t.Fatal(err)
				}
				if !utf8.Valid(data) {
					t.Fatal("SKILL.md is not valid UTF-8")
				}
				fm, body := splitSkillFrontmatter(t, string(data))
				if fm["name"] != name {
					t.Fatalf("frontmatter name = %v, directory = %s", fm["name"], name)
				}
				description, ok := fm["description"].(string)
				if !ok || strings.TrimSpace(description) == "" {
					t.Fatal("frontmatter description is missing")
				}
				if n := utf8.RuneCountInString(description); n > 1024 {
					t.Fatalf("description is %d characters; Codex limit is 1024", n)
				}
				if strings.ContainsAny(description, "<>") {
					t.Fatal("description contains an angle bracket, which Codex rejects")
				}
				for key := range fm {
					if !allowedFrontmatter[key] {
						t.Errorf("unsupported Codex SKILL.md frontmatter field %q", key)
					}
				}
				for _, forbidden := range []string{"$ARGUMENTS", "CLAUDE_PLUGIN_ROOT"} {
					if strings.Contains(body, forbidden) {
						t.Errorf("body retains host-only token %q", forbidden)
					}
				}

				metaPath := filepath.Join(filepath.Dir(skillPath), "agents", "openai.yaml")
				metaData, err := os.ReadFile(metaPath)
				if err != nil {
					if explicitOnly[name] || !os.IsNotExist(err) {
						t.Fatalf("required metadata missing: %v", err)
					}
				} else {
					var meta openAIMetadata
					if err := yaml.Unmarshal(metaData, &meta); err != nil {
						t.Fatalf("invalid agents/openai.yaml: %v", err)
					}
					if strings.TrimSpace(meta.Interface.DisplayName) == "" {
						t.Error("interface.display_name is empty")
					}
					shortLen := utf8.RuneCountInString(meta.Interface.ShortDescription)
					if shortLen < 25 || shortLen > 64 {
						t.Errorf("interface.short_description is %d characters; want 25..64", shortLen)
					}
					if meta.Policy.AllowImplicitInvocation == nil {
						t.Error("policy.allow_implicit_invocation is missing")
					} else if explicitOnly[name] == *meta.Policy.AllowImplicitInvocation {
						t.Errorf("allow_implicit_invocation = %t, want %t", *meta.Policy.AllowImplicitInvocation, !explicitOnly[name])
					}
				}

				for _, ref := range skillSupportRefRe.FindAllString(body, -1) {
					ref = strings.TrimRight(ref, ".,;:)")
					if _, err := os.Stat(filepath.Join(filepath.Dir(skillPath), filepath.FromSlash(ref))); err != nil {
						t.Errorf("dangling skill-local support reference %q", ref)
					}
				}
			})
		}
	}

	for name := range expected {
		if !seen[name] {
			t.Errorf("Codex artifact %q was not installed from any bundle", name)
		}
	}
	for name := range seen {
		if !expected[name] {
			t.Errorf("installed unexpected Codex skill %q", name)
		}
	}
}
