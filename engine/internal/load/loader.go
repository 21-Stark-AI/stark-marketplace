package load

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// Load walks catalogDir in sorted order and returns the parsed Catalog.
// Pure: no clock/network/env. Bytes are normalized to LF on read.
func Load(catalogDir string) (*model.Catalog, error) {
	entries, err := os.ReadDir(catalogDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	cat := &model.Catalog{}
	for _, name := range names {
		b, err := loadBundle(catalogDir, name)
		if err != nil {
			return nil, fmt.Errorf("bundle %s: %w", name, err)
		}
		cat.Bundles = append(cat.Bundles, b)
	}
	return cat, nil
}

func loadBundle(catalogDir, name string) (*model.Bundle, error) {
	bundleDir := filepath.Join(catalogDir, name)
	raw, err := readLF(filepath.Join(bundleDir, "bundle.yaml"))
	if err != nil {
		return nil, err
	}
	var b model.Bundle
	if err := yaml.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("bundle.yaml: %w", err)
	}
	b.SourcePath = filepath.ToSlash(bundleDir)
	if b.Name != name {
		return nil, fmt.Errorf("bundle.yaml name %q != dir %q", b.Name, name)
	}

	// type dirs in deterministic order
	for _, t := range model.AllArtifactTypes() {
		dir := filepath.Join(bundleDir, typeDir(t))
		files, err := os.ReadDir(dir)
		if err != nil {
			// A missing type dir is normal (a bundle rarely has all five). Any other
			// error (permission, ENOTDIR when the name is a regular file) is a real
			// fault and must fail closed rather than be read as "no artifacts".
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read %s: %w", typeDir(t), err)
			}
			files = nil
		}
		var fnames []string
		for _, f := range files {
			// Only canonical artifact files are loaded: `.md` for the markdown types,
			// `.yaml`/`.yml` for mcp. Stray files (.DS_Store, .json, README.txt, editor
			// swap files) are skipped so they never become phantom YAML artifacts.
			if f.IsDir() || !isArtifactFile(t, f.Name()) {
				continue
			}
			fnames = append(fnames, f.Name())
		}
		sort.Strings(fnames)
		for _, fn := range fnames {
			a, err := loadArtifact(filepath.Join(dir, fn), &b)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: %w", typeDir(t), fn, err)
			}
			b.Artifacts = append(b.Artifacts, a)
		}
	}
	return &b, nil
}

func loadArtifact(path string, b *model.Bundle) (*model.Artifact, error) {
	data, err := readLF(path)
	if err != nil {
		return nil, err
	}
	var a model.Artifact
	if strings.HasSuffix(path, ".md") {
		fm, body, err := splitFrontmatter(data)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(fm, &a); err != nil {
			return nil, err
		}
		a.Body = body
		a.Raw = rawForSchema(fm)
	} else { // .yaml (mcp)
		if err := yaml.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		a.Raw = rawForSchema(data)
	}
	a.Bundle = b.Name
	a.SourcePath = filepath.ToSlash(path)
	inherit(&a, b)
	return &a, nil
}

// inherit fills unset artifact fields from the bundle (spec §5.2).
func inherit(a *model.Artifact, b *model.Bundle) {
	if a.Category == "" {
		a.Category = b.Category
	}
	if len(a.Tags) == 0 {
		a.Tags = append([]string(nil), b.Tags...)
	}
	if a.Maturity == "" {
		a.Maturity = b.Maturity
	}
	if len(a.Runtimes) == 0 {
		a.Runtimes = append([]model.Runtime(nil), b.Runtimes...)
	}
}

func typeDir(t model.ArtifactType) string {
	switch t {
	case model.TypeMCP:
		return "mcp"
	default:
		return string(t) + "s" // skills, prompts, commands, agents
	}
}

// isArtifactFile reports whether name is a canonical artifact file for type t:
// `.yaml`/`.yml` for mcp, `.md` for the markdown-bodied types. Anything else is
// a stray file to skip (keeps loadArtifact's md/yaml routing well-defined too).
func isArtifactFile(t model.ArtifactType, name string) bool {
	if t == model.TypeMCP {
		return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
	}
	return strings.HasSuffix(name, ".md")
}

func readLF(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), nil
}

// rawForSchema decodes YAML frontmatter into a JSON-faithful value for schema
// validation (jsonschema/v6 is JSON-typed). Returns nil on any decode/encode
// error; the schema rule reports nil Raw as an unparseable-frontmatter error.
func rawForSchema(yamlBytes []byte) any {
	var v any
	if err := yaml.Unmarshal(yamlBytes, &v); err != nil {
		return nil
	}
	j, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	out, err := jsonschema.UnmarshalJSON(bytes.NewReader(j))
	if err != nil {
		return nil
	}
	return out
}
