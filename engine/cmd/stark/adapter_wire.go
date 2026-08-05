package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/21StarkCom/bifrost/engine/internal/adapter"
	"github.com/21StarkCom/bifrost/engine/internal/adapter/codex"
	"github.com/21StarkCom/bifrost/engine/internal/adapter/registry"
	"github.com/21StarkCom/bifrost/engine/internal/aggregate"
	"github.com/21StarkCom/bifrost/engine/internal/build"
	"github.com/21StarkCom/bifrost/engine/internal/indexio"
	"github.com/21StarkCom/bifrost/engine/internal/install"
	"github.com/21StarkCom/bifrost/engine/internal/installplan"
	"github.com/21StarkCom/bifrost/engine/internal/load"
	"github.com/21StarkCom/bifrost/engine/internal/model"
)

// catalogAdapter is the production installplan.Adapter (Task 14). It renders a source bundle's
// runtime target IN MEMORY and maps each declared output to its real payload. This realizes
// spec §5.1: the codex/gemini dist trees are produced on `stark install`, never committed —
// only dist/claude/ is committed (for the native CC marketplace). The artifact is rendered in
// a single-artifact sub-bundle so the target's output is exactly this artifact's files (a
// bundle with several MCP servers would otherwise collide on config.toml).
type catalogAdapter struct {
	catalogDir string
	targets    map[model.Runtime]adapter.Target

	// assetsSource is the shared stark-skills snapshot (vendor/stark-skills) and
	// pluginAssetsRoot holds the per-bundle overlays (vendor/plugins/<bundle>) —
	// the same two inputs `stark build` vendors into dist/claude/<bundle>/. Empty
	// = nothing to vendor (a catalog checked out without them).
	assetsSource     string
	pluginAssetsRoot string

	mu     sync.Mutex
	cat    *model.Catalog
	shared map[string][]byte            // assetsSource, read once
	plugin map[string]map[string][]byte // bundle -> overlay, read once
}

func newCatalogAdapter(catalogDir string) *catalogAdapter {
	return &catalogAdapter{catalogDir: catalogDir, targets: registry.All()}
}

// catalog loads the source tree once (lazily) and caches it.
func (c *catalogAdapter) catalog() (*model.Catalog, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cat != nil {
		return c.cat, nil
	}
	cat, err := load.Load(c.catalogDir)
	if err != nil {
		return nil, fmt.Errorf("load catalog %q: %w", c.catalogDir, err)
	}
	c.cat = cat
	return cat, nil
}

func (c *catalogAdapter) Adapt(bundle string, a *indexio.ArtifactDetail, rt model.Runtime) ([]installplan.AdaptedFile, error) {
	outs, ok := a.Outputs[rt]
	if !ok || len(outs) == 0 {
		return nil, fmt.Errorf("artifact %s/%s does not target %s", bundle, a.Name, rt)
	}
	tgt, ok := c.targets[rt]
	if !ok {
		return nil, fmt.Errorf("no adapter target for runtime %s", rt)
	}
	rendered, err := c.renderArtifact(bundle, a, tgt)
	if err != nil {
		return nil, err
	}
	res := make([]installplan.AdaptedFile, 0, len(outs))
	for _, o := range outs {
		content, ok := rendered[o.Path]
		if !ok {
			return nil, fmt.Errorf("%s/%s@%s: declared output %q was not rendered", bundle, a.Name, rt, o.Path)
		}
		payload, err := shapePayload(content, o)
		if err != nil {
			return nil, fmt.Errorf("%s/%s@%s %s: %w", bundle, a.Name, rt, o.Path, err)
		}
		res = append(res, installplan.AdaptedFile{
			Path: o.Path, Kind: o.Kind, Key: o.Key, Sentinel: o.Sentinel,
			Emulated: o.Emulated, Payload: payload,
		})
	}
	return res, nil
}

// renderArtifact renders just one artifact by rendering a single-artifact copy of its bundle.
func (c *catalogAdapter) renderArtifact(bundle string, a *indexio.ArtifactDetail, tgt adapter.Target) (map[string]string, error) {
	cat, err := c.catalog()
	if err != nil {
		return nil, err
	}
	var src *model.Bundle
	for _, b := range cat.Bundles {
		if b.Name == bundle {
			src = b
			break
		}
	}
	if src == nil {
		return nil, fmt.Errorf("bundle %q not found in catalog", bundle)
	}
	var art *model.Artifact
	for _, ar := range src.Artifacts {
		if ar.Name == a.Name && ar.Type == a.Type {
			art = ar
			break
		}
	}
	if art == nil {
		return nil, fmt.Errorf("artifact %s/%s (%s) not found in catalog", bundle, a.Name, a.Type)
	}
	sub := *src // shallow copy of bundle metadata
	sub.Artifacts = []*model.Artifact{art}
	files, _, err := tgt.Render(&sub)
	if err != nil {
		return nil, fmt.Errorf("render %s/%s@%s: %w", bundle, a.Name, tgt.Runtime(), err)
	}
	out := make(map[string]string, len(files))
	for _, f := range files {
		out[f.Path] = string(f.Content)
	}
	return out, nil
}

// shapePayload converts a rendered file into the payload the executor's merge expects:
//   - file:         the whole file body
//   - mergeTOMLKey: the complete [key] block (MergeTOMLKey splices it; generalized to accept
//     the artifact's own header + subtables)
//   - mergeJSONKey: the JSON VALUE at the dotted key (the executor json.Unmarshals the payload
//     then MergeJSONKey-s it under the key, so the wrapper object must be stripped)
//   - sentinel:     the block body, markers stripped (MergeSentinel re-wraps it)
func shapePayload(content string, o indexio.Output) (string, error) {
	switch o.Kind {
	case "file":
		return content, nil
	case "mergeTOMLKey":
		// The rendered fragment may carry a bare parent header (e.g. [mcp_servers]) plus the
		// keyed table; pull out only the [o.Key] block (+ its subtables) as the managed payload.
		block := install.ExtractTOMLTable([]byte(content), o.Key)
		if block == "" {
			return "", fmt.Errorf("rendered TOML has no [%s] table", o.Key)
		}
		return block, nil
	case "mergeJSONKey":
		return extractJSONValue(content, o.Key)
	case "sentinel":
		return sentinelBody(content, o.Sentinel)
	default:
		return "", fmt.Errorf("unknown output kind %q", o.Kind)
	}
}

// extractJSONValue navigates the dotted key in a rendered JSON document and returns the value
// at that path re-marshaled with 2-space indent (the executor merges it back under the key).
func extractJSONValue(content, dottedKey string) (string, error) {
	var root any
	if err := json.Unmarshal([]byte(content), &root); err != nil {
		return "", fmt.Errorf("rendered JSON invalid: %w", err)
	}
	cur := root
	for _, p := range strings.Split(dottedKey, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("rendered JSON has no object at %q", dottedKey)
		}
		v, ok := m[p]
		if !ok {
			return "", fmt.Errorf("rendered JSON missing key %q", dottedKey)
		}
		cur = v
	}
	b, err := json.MarshalIndent(cur, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// sentinelBody recovers the clean inner body of one emulation section from a rendered shared
// file. The render-time markers carry a digest (`<!-- stark:begin <id>@<digest> -->`), so we
// parse with aggregate.Parse (the single source of truth for that format) rather than matching
// the install-side (digest-free) markers — otherwise the markers leak into the payload and
// install.MergeSentinel double-wraps them, corrupting the file. id is "<bundle>/<name>".
func sentinelBody(content, id string) (string, error) {
	for _, s := range aggregate.Parse(content) {
		if s.Bundle+"/"+s.Name == id {
			return s.Content, nil
		}
	}
	return "", fmt.Errorf("rendered output has no sentinel section %q", id)
}

// assetPathFor returns the runtime's mapper from a bundle-relative asset path (its
// place in dist/claude/<bundle>/) to a path in that runtime's tree, or nil when the
// runtime has no asset layout.
//
// Claude is deliberately nil: its distribution path is the committed dist/claude/
// plugin tree, which `stark build` already vendors — a second copy written by
// `stark install` would be unmanaged duplication, not a fix.
func assetPathFor(rt model.Runtime) func(bundle, rel string) string {
	if rt == model.RuntimeCodex {
		return codex.AssetPath
	}
	return nil
}

// BundleAssets implements installplan.AssetProvider: the vendored stark-skills
// snapshot plus this bundle's plugin overlay, mapped into the runtime's tree. Skill
// bodies reference these by path (${CLAUDE_PLUGIN_ROOT}/tools/x.ts,
// ../../standards/help.md); without them every such reference dangles.
func (c *catalogAdapter) BundleAssets(bundle string, rt model.Runtime) ([]installplan.AdaptedFile, error) {
	mapPath := assetPathFor(rt)
	if mapPath == nil {
		return nil, nil
	}
	shared, plugin, err := c.assets(bundle)
	if err != nil {
		return nil, err
	}
	// Shared snapshot first, then the bundle's own overlay — same precedence as
	// build.Build, so e.g. stark-gh's {draft} config.json wins over the global one.
	merged := make(map[string][]byte, len(shared)+len(plugin))
	for rel, content := range shared {
		merged[rel] = content
	}
	for rel, content := range plugin {
		merged[rel] = content
	}
	rels := make([]string, 0, len(merged))
	for rel := range merged {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	out := make([]installplan.AdaptedFile, 0, len(rels))
	for _, rel := range rels {
		out = append(out, installplan.AdaptedFile{
			Path:    mapPath(bundle, rel),
			Kind:    "file",
			Payload: string(build.ToLF(merged[rel])),
		})
	}
	return out, nil
}

// assets reads (once) the shared snapshot and this bundle's overlay. A missing
// directory is not an error — it just means there is nothing to vendor.
func (c *catalogAdapter) assets(bundle string) (map[string][]byte, map[string][]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.shared == nil {
		c.shared = map[string][]byte{}
		if c.assetsSource != "" && dirExists(c.assetsSource) {
			v, err := build.VendorAssets(c.assetsSource)
			if err != nil {
				return nil, nil, fmt.Errorf("vendor assets from %s: %w", c.assetsSource, err)
			}
			c.shared = v
		}
	}
	if c.plugin == nil {
		c.plugin = map[string]map[string][]byte{}
	}
	overlay, ok := c.plugin[bundle]
	if !ok {
		overlay = map[string][]byte{}
		if c.pluginAssetsRoot != "" {
			dir := filepath.Join(c.pluginAssetsRoot, bundle)
			if dirExists(dir) {
				v, err := build.VendorAssets(dir)
				if err != nil {
					return nil, nil, fmt.Errorf("plugin assets from %s: %w", dir, err)
				}
				overlay = v
			}
		}
		c.plugin[bundle] = overlay
	}
	return c.shared, overlay, nil
}

// realAdapter returns the production adapter, rendering from the given catalog directory.
// assetsSource / pluginAssetsRoot are the vendor roots to install alongside the artifacts;
// empty disables asset vendoring.
func realAdapter(catalogDir, assetsSource, pluginAssetsRoot string) installplan.Adapter {
	c := newCatalogAdapter(catalogDir)
	c.assetsSource = assetsSource
	c.pluginAssetsRoot = pluginAssetsRoot
	return c
}
