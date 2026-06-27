// Package build orchestrates the slice-2 pipeline: render the Claude target for
// every bundle into dist/claude/, build the lean index + per-bundle detail, and
// compute the divergence budget (spec §4.3, §7.1). Output is a path->bytes map so
// the writer/checker can diff or write deterministically.
package build

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/claude"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/canonjson"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/index"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/marketplace"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// toLF normalizes content to LF so the in-memory set, the on-disk write, and the
// drift check all hash identical bytes regardless of any CR in source bodies
// (F-Cov#7 / spec §7.6).
func toLF(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(b, []byte("\r"), []byte("\n"))
}

// Output is the full set of generated repo-relative files plus surfaced signals.
type Output struct {
	Files            map[string][]byte // repo-relative path -> content
	Warnings         []string
	DivergenceBudget string // e.g. "diverged 0 / 2 = 0.0%"
}

// Options tunes the build.
type Options struct {
	// AssetsSource is a directory whose entire contents are vendored verbatim
	// into every rendered Claude bundle (dist/claude/<bundle>/<relpath>), making
	// each plugin self-contained. It is the normalized immutable-asset snapshot
	// of stark-skills: tools/, prompts/, standards/, scripts/, config.json,
	// forge_heuristics.json. Empty = skip vendoring (skills resolve from
	// ~/.claude/code-review via install.sh instead). The snapshot is produced by
	// the generator; filtering (tests/fixtures/node_modules) is its job, not the
	// engine's — the engine copies byte-for-byte for determinism.
	AssetsSource string

	// PluginAssetsRoot is a directory holding per-bundle plugin asset snapshots
	// (<root>/<bundle>/<relpath>, produced by `stark sync` from plugins/<bundle>).
	// For each bundle with a subdir here, those files are layered into THAT
	// bundle's dist tree AFTER the shared AssetsSource — so a plugin's own
	// config.json + its plugin-specific .ts tools (e.g. stark-gh's gh_*.ts /
	// {draft} config) override the shared snapshot for that bundle only. Empty =
	// no per-bundle layering. Like AssetsSource, copied byte-for-byte.
	PluginAssetsRoot string
}

// generatedRoots are the repo-relative trees this build owns and fully regenerates.
// .claude-plugin holds the repo-root CC marketplace manifest (spec §8).
var generatedRoots = []string{"dist/claude", "index.json", "bundles", ".claude-plugin"}

// Build runs the pipeline over a loaded catalog.
func Build(cat *model.Catalog, opts Options) (Output, error) {
	out := Output{Files: map[string][]byte{}}
	tgt := claude.New()

	// Read the immutable-asset snapshot once; it is vendored identically into
	// every bundle so each rendered plugin is self-contained.
	var vendored map[string][]byte
	if opts.AssetsSource != "" {
		v, err := vendorAssets(opts.AssetsSource)
		if err != nil {
			return Output{}, fmt.Errorf("vendor assets from %s: %w", opts.AssetsSource, err)
		}
		vendored = v
	}

	// Per-bundle plugin assets (vendor/plugins/<bundle>): read once, keyed by
	// bundle name, layered over the shared snapshot for the owning bundle only.
	pluginAssets := map[string]map[string][]byte{}
	if opts.PluginAssetsRoot != "" {
		for _, b := range cat.Bundles {
			dir := filepath.Join(opts.PluginAssetsRoot, b.Name)
			if fi, statErr := os.Stat(dir); statErr != nil || !fi.IsDir() {
				continue
			}
			pa, err := vendorAssets(dir)
			if err != nil {
				return Output{}, fmt.Errorf("plugin assets from %s: %w", dir, err)
			}
			pluginAssets[b.Name] = pa
		}
	}

	totalArtifacts := 0
	diverged := 0
	for _, b := range cat.Bundles {
		totalArtifacts += len(b.Artifacts)
		files, findings, err := tgt.Render(b)
		if err != nil {
			return Output{}, fmt.Errorf("render %s: %w", b.Name, err)
		}
		// Shared vendored assets first, then this bundle's own plugin assets
		// (overriding the shared snapshot — e.g. stark-gh's {draft} config.json +
		// gh_*.ts tools), then rendered artifacts win on any remaining collision.
		for rel, content := range vendored {
			out.Files["dist/claude/"+b.Name+"/"+rel] = toLF(content)
		}
		for rel, content := range pluginAssets[b.Name] {
			out.Files["dist/claude/"+b.Name+"/"+rel] = toLF(content)
		}
		for _, f := range files {
			out.Files["dist/claude/"+b.Name+"/"+f.Path] = toLF(f.Content)
		}
		// findings is a flat []adapter.Finding (CC-1). Surface warnings; count the
		// divergence findings (Msg prefixed "diverged:") for the budget.
		for _, fd := range findings {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%s [%s] %s", fd.Where, fd.Level, fd.Msg))
			if strings.HasPrefix(fd.Msg, "diverged:") {
				diverged++
			}
		}
	}

	// index.json + bundles/<name>.json
	idx, details, err := index.Build(cat)
	if err != nil {
		return Output{}, err
	}
	idxBytes, err := canonjson.Marshal(idx)
	if err != nil {
		return Output{}, err
	}
	out.Files["index.json"] = idxBytes
	names := make([]string, 0, len(details))
	for n := range details {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		db, err := canonjson.Marshal(details[n])
		if err != nil {
			return Output{}, err
		}
		out.Files["bundles/"+n+".json"] = db
	}

	// CC marketplace manifest (spec §8): one plugins[] entry per bundle, committed
	// under dist/claude so `/plugin marketplace add` resolves it. Emitted into the
	// generated set so the existing drift gate covers it — no separate gate.
	// The manifest is committed at the REPO ROOT (.claude-plugin/marketplace.json):
	// `/plugin marketplace add 21-Stark-AI/stark-marketplace` looks for the manifest
	// at the repo root, and CC resolves each entry's relative `source` against the
	// marketplace root (= the dir containing .claude-plugin/ = repo root), so a
	// source of "./dist/claude/<bundle>" resolves to the committed bundle tree.
	mani, err := marketplace.Marshal(marketplace.Generate(cat, marketplace.Options{
		Name:     "stark-marketplace",
		Owner:    marketplace.Owner{Name: "21 Stark AI", Email: "engineering@21stark.com"},
		DistRoot: "./dist/claude",
	}))
	if err != nil {
		return Output{}, err
	}
	out.Files[marketplace.ManifestRelPath] = mani

	pct := 0.0
	if totalArtifacts > 0 {
		pct = 100 * float64(diverged) / float64(totalArtifacts)
	}
	out.DivergenceBudget = fmt.Sprintf("diverged %d / %d = %.1f%%", diverged, totalArtifacts, pct)
	return out, nil
}

// Write cleans the generated roots under repoRoot and writes every file in out.
// Paths use `/` and are joined under repoRoot with the OS separator. Every file's
// content is normalized to LF on the way out (F-Cov#7): the determinism contract
// (spec §7.6) forbids CRLF in generated files regardless of host OS or any CR that
// slipped through a body/string value.
func Write(repoRoot string, out Output) error {
	for _, r := range generatedRoots {
		_ = os.RemoveAll(filepath.Join(repoRoot, filepath.FromSlash(r)))
	}
	paths := sortedKeys(out.Files)
	for _, p := range paths {
		abs := filepath.Join(repoRoot, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		content := bytes.ReplaceAll(out.Files[p], []byte("\r\n"), []byte("\n"))
		content = bytes.ReplaceAll(content, []byte("\r"), []byte("\n"))
		out.Files[p] = content // keep the in-memory set byte-identical to disk for Check
		if err := os.WriteFile(abs, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Check compares the expected build output against what is on disk under
// repoRoot. It returns a sorted list of drifted repo-relative paths (missing,
// changed, or unexpected-extra within the generated roots).
func Check(repoRoot string, out Output) ([]string, error) {
	var drift []string
	// expected: must exist and match
	for _, p := range sortedKeys(out.Files) {
		abs := filepath.Join(repoRoot, filepath.FromSlash(p))
		got, err := os.ReadFile(abs)
		if os.IsNotExist(err) {
			drift = append(drift, p+" (missing)")
			continue
		}
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(got, out.Files[p]) {
			drift = append(drift, p+" (changed)")
		}
	}
	// extras: any on-disk file under a generated root not in the expected set
	for _, r := range generatedRoots {
		base := filepath.Join(repoRoot, filepath.FromSlash(r))
		_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(repoRoot, path)
			rel = filepath.ToSlash(rel)
			if _, ok := out.Files[rel]; !ok {
				drift = append(drift, rel+" (unexpected)")
			}
			return nil
		})
	}
	sort.Strings(drift)
	return drift, nil
}

// vendorAssets reads every regular file under src and returns a map of
// slash-relative path -> content, to be copied verbatim into each bundle's dist
// tree. Directories and non-regular files (symlinks) are skipped; the snapshot
// is expected to be pre-filtered by the generator (no tests/fixtures/node_modules).
func vendorAssets(src string) (map[string][]byte, error) {
	info, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("assets source %q is not a directory", src)
	}
	out := map[string][]byte{}
	err = filepath.WalkDir(src, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func sortedKeys(m map[string][]byte) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
