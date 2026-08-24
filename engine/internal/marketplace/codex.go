package marketplace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	codexadapter "github.com/21StarkCom/bifrost/engine/internal/adapter/codex"
	"github.com/21StarkCom/bifrost/engine/internal/model"
)

// CodexManifestRelPath is the repository marketplace manifest consumed by
// Codex. It is deliberately separate from the legacy-compatible Claude
// marketplace manifest: each host points at its own committed plugin tree.
const CodexManifestRelPath = ".agents/plugins/marketplace.json"

const (
	defaultCodexCategory = "Productivity"
	defaultCodexOwner    = "21 Stark AI"
	defaultCodexWebsite  = "https://marketplace.21stark.com/"
	codexRepositoryURL   = "https://github.com/21StarkCom/bifrost"
)

var strictSemverRE = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

// CodexAuthor is the publisher identity accepted by .codex-plugin/plugin.json.
type CodexAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// CodexPluginInterface is the required install-surface metadata for a native
// Codex plugin. Fields without a real repository asset or legal URL are omitted
// instead of pointing at placeholders.
type CodexPluginInterface struct {
	DisplayName      string   `json:"displayName"`
	ShortDescription string   `json:"shortDescription"`
	LongDescription  string   `json:"longDescription"`
	DeveloperName    string   `json:"developerName"`
	Category         string   `json:"category"`
	Capabilities     []string `json:"capabilities"`
	WebsiteURL       string   `json:"websiteURL,omitempty"`
	DefaultPrompt    []string `json:"defaultPrompt"`
}

// CodexPluginManifest is one generated bundle-level plugin manifest.
type CodexPluginManifest struct {
	Name        string               `json:"name"`
	Version     string               `json:"version"`
	Description string               `json:"description"`
	Author      CodexAuthor          `json:"author"`
	Homepage    string               `json:"homepage,omitempty"`
	Repository  string               `json:"repository,omitempty"`
	Keywords    []string             `json:"keywords,omitempty"`
	Skills      string               `json:"skills,omitempty"`
	MCPServers  string               `json:"mcpServers,omitempty"`
	Interface   CodexPluginInterface `json:"interface"`
}

// CodexMarketplaceSource is a repository-local plugin source. Paths are
// resolved against the marketplace root (the repository root), not the nested
// .agents/plugins directory that contains the manifest.
type CodexMarketplaceSource struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}

type CodexMarketplacePolicy struct {
	Installation   string `json:"installation"`
	Authentication string `json:"authentication"`
}

type CodexMarketplacePlugin struct {
	Name     string                 `json:"name"`
	Source   CodexMarketplaceSource `json:"source"`
	Policy   CodexMarketplacePolicy `json:"policy"`
	Category string                 `json:"category"`
}

type CodexMarketplaceInterface struct {
	DisplayName string `json:"displayName"`
}

// CodexMarketplaceManifest is the native repo/team marketplace schema.
type CodexMarketplaceManifest struct {
	Name      string                    `json:"name"`
	Interface CodexMarketplaceInterface `json:"interface"`
	Plugins   []CodexMarketplacePlugin  `json:"plugins"`
}

type CodexMarketplaceOptions struct {
	Name        string
	DisplayName string
	DistRoot    string
}

// GenerateCodexPlugin projects bundle metadata into a validation-ready native
// Codex manifest. version is the Bifrost release version, intentionally shared
// by all bundle packages produced by one build.
func GenerateCodexPlugin(b *model.Bundle, version string) (CodexPluginManifest, error) {
	if b == nil {
		return CodexPluginManifest{}, fmt.Errorf("codex plugin: nil bundle")
	}
	if !strictSemverRE.MatchString(version) {
		return CodexPluginManifest{}, fmt.Errorf("codex plugin %s: version %q is not strict semver", b.Name, version)
	}

	description := strings.TrimSpace(b.Description)
	if description == "" {
		description = displayCodexName(b.Name) + " plugin"
	}
	description = codexadapter.TranslateInvocationReferences(description, b)
	developer := strings.TrimSpace(b.Owner.Name)
	if developer == "" {
		developer = defaultCodexOwner
	}
	homepage := strings.TrimSpace(b.Homepage)
	if homepage == "" {
		homepage = defaultCodexWebsite
	}
	category := codexCategory(b.Category)
	displayName := displayCodexName(b.Name)

	manifest := CodexPluginManifest{
		Name:        b.Name,
		Version:     version,
		Description: description,
		Author: CodexAuthor{
			Name:  developer,
			Email: strings.TrimSpace(b.Owner.Email),
			URL:   "https://github.com/21StarkCom",
		},
		Homepage:   homepage,
		Repository: codexRepositoryURL,
		Keywords:   append([]string(nil), b.Tags...),
		Interface: CodexPluginInterface{
			DisplayName:      displayName,
			ShortDescription: shortCodexDescription(description),
			LongDescription:  description,
			DeveloperName:    developer,
			Category:         category,
			Capabilities:     []string{"Interactive", "Read", "Write"},
			WebsiteURL:       homepage,
			DefaultPrompt:    []string{"Use " + displayName + " for this task."},
		},
	}
	for _, artifact := range b.Artifacts {
		if !artifactTargetsRuntime(artifact, model.RuntimeCodex) {
			continue
		}
		if artifact.Type == model.TypeMCP {
			manifest.MCPServers = "./.mcp.json"
		} else {
			manifest.Skills = "./skills/"
		}
	}
	return manifest, nil
}

// GenerateCodexMarketplace emits one entry for every bundle that has at least
// one Codex-targeted artifact. Entries are sorted by name independently of the
// input catalog order.
func GenerateCodexMarketplace(cat *model.Catalog, opts CodexMarketplaceOptions) CodexMarketplaceManifest {
	name := opts.Name
	if name == "" {
		name = "bifrost"
	}
	displayName := opts.DisplayName
	if displayName == "" {
		displayName = "21 Stark"
	}
	distRoot := strings.TrimSuffix(opts.DistRoot, "/")
	if distRoot == "" {
		distRoot = "./dist/codex-plugins"
	}

	bundles := append([]*model.Bundle(nil), cat.Bundles...)
	sort.Slice(bundles, func(i, j int) bool { return bundles[i].Name < bundles[j].Name })

	m := CodexMarketplaceManifest{
		Name:      name,
		Interface: CodexMarketplaceInterface{DisplayName: displayName},
		Plugins:   []CodexMarketplacePlugin{},
	}
	for _, b := range bundles {
		if !bundleTargetsRuntime(b, model.RuntimeCodex) {
			continue
		}
		m.Plugins = append(m.Plugins, CodexMarketplacePlugin{
			Name: b.Name,
			Source: CodexMarketplaceSource{
				Source: "local",
				Path:   distRoot + "/" + b.Name,
			},
			Policy: CodexMarketplacePolicy{
				Installation:   "AVAILABLE",
				Authentication: "ON_INSTALL",
			},
			Category: codexCategory(b.Category),
		})
	}
	return m
}

func bundleTargetsRuntime(b *model.Bundle, rt model.Runtime) bool {
	for _, a := range b.Artifacts {
		if artifactTargetsRuntime(a, rt) {
			return true
		}
	}
	return false
}

func artifactTargetsRuntime(a *model.Artifact, rt model.Runtime) bool {
	for _, target := range a.Runtimes {
		if target == rt {
			return true
		}
	}
	return false
}

func codexCategory(category string) string {
	category = strings.TrimSpace(category)
	if category == "" {
		return defaultCodexCategory
	}
	runes := []rune(category)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func displayCodexName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	for i, part := range parts {
		runes := []rune(part)
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	if len(parts) == 0 {
		return "Stark Plugin"
	}
	return strings.Join(parts, " ")
}

func shortCodexDescription(description string) string {
	runes := []rune(strings.Join(strings.Fields(description), " "))
	if len(runes) <= 96 {
		return string(runes)
	}
	cut := 96
	for cut > 48 && !unicode.IsSpace(runes[cut]) {
		cut--
	}
	return strings.TrimSpace(string(runes[:cut]))
}

// MarshalCodex serializes either native Codex manifest deterministically with
// two-space indentation, LF, no HTML escaping, and one trailing newline.
func MarshalCodex(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
