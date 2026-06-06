package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
	"gopkg.in/yaml.v3"
)

// pluginManifest mirrors plugins/stark-gh/.claude-plugin/plugin.json.
type pluginManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
}

// importPlugin maps plugins/stark-gh (commands + optional mcp) into the bundle.
// Missing plugin dir is not an error (a skill-only import is valid).
func importPlugin(from, bundle string, res *ImportResult) error {
	pluginDir := filepath.Join(from, "plugins", "stark-gh")
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return nil
	}
	if err := seedBundleFromManifest(pluginDir, res); err != nil {
		return err
	}
	if err := importPluginCommands(pluginDir, bundle, res); err != nil {
		return err
	}
	return importPluginMCP(pluginDir, bundle, res)
}

// seedBundleFromManifest fills bundle description/owner from plugin.json when present.
func seedBundleFromManifest(pluginDir string, res *ImportResult) error {
	path := filepath.Join(pluginDir, ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var pm pluginManifest
	if err := json.Unmarshal(data, &pm); err != nil {
		return fmt.Errorf("plugin.json: %w", err)
	}
	b := res.Bundle
	if pm.Description != "" {
		b.Description = pm.Description
	}
	if pm.Author.Name != "" {
		b.Owner = model.Owner{Name: pm.Author.Name, Email: pm.Author.Email}
	}
	return nil
}

func importPluginCommands(pluginDir, bundle string, res *ImportResult) error {
	dir := filepath.Join(pluginDir, "commands")
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var names []string
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".md" {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)
	for _, fn := range names {
		a, err := mapCommandFile(filepath.Join(dir, fn), bundle, res)
		if err != nil {
			return fmt.Errorf("command %s: %w", fn, err)
		}
		res.Bundle.Artifacts = append(res.Bundle.Artifacts, a)
	}
	return nil
}

func mapCommandFile(path, bundle string, res *ImportResult) (*model.Artifact, error) {
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
	a := &model.Artifact{Type: model.TypeCommand, Bundle: bundle, Body: cleanBody(body)}
	mapCommonFrontmatter(a, raw)
	// fall back to the file basename when `name:` is absent, so commands never write a name-less
	// artifact that fails validate.
	nameDerived := false
	if a.Name == "" {
		a.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		nameDerived = true
	}
	where := bundle + "/command/" + a.Name
	if nameDerived {
		res.note(where, "name", "name derived from the source filename (frontmatter had none) — confirm")
	}
	if sanitized {
		res.note(where, "argument-hint", "argument-hint was reformatted from a loose/unquoted source value — verify it")
	}
	noteUnmappedFields(raw, res, where)
	applyArtifactDefaults(a, res, where)
	return a, nil
}

func importPluginMCP(pluginDir, bundle string, res *ImportResult) error {
	dir := filepath.Join(pluginDir, "mcp")
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil // real stark-gh has no mcp/ — that's fine
	}
	if err != nil {
		return err
	}
	var names []string
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".yaml" {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)
	for _, fn := range names {
		a, err := mapMCPFile(filepath.Join(dir, fn), bundle, res)
		if err != nil {
			return fmt.Errorf("mcp %s: %w", fn, err)
		}
		res.Bundle.Artifacts = append(res.Bundle.Artifacts, a)
	}
	return nil
}

func mapMCPFile(path, bundle string, res *ImportResult) (*model.Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var a model.Artifact
	if err := yaml.Unmarshal(normalizeLF(data), &a); err != nil {
		return nil, err
	}
	a.Type = model.TypeMCP
	a.Bundle = bundle
	where := bundle + "/mcp/" + a.Name
	applyArtifactDefaults(&a, res, where)
	return &a, nil
}
