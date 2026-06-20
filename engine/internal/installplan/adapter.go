package installplan

import (
	"fmt"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/indexio"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// AdaptedFile is one concrete file the executor must write/merge.
type AdaptedFile struct {
	Path     string // runtime-relative path under the dest root (forward slashes)
	Kind     string // file | mergeJSONKey | mergeTOMLKey | sentinel
	Key      string // dotted key for merge kinds
	Sentinel string // bundle/name id for sentinel kind
	Emulated bool
	Payload  string // file body, or the managed subtree/sentinel block to merge in
}

// Adapter renders an artifact's per-runtime output. `bundle` is the owning bundle name —
// the real adapter (cmd/stark) needs it to render the source bundle via the runtime target,
// since ArtifactDetail does not carry its own bundle. The real implementation lives in
// cmd/stark and renders slice-03's targets in-memory (spec §5.1: codex/gemini dist is built
// on install, never committed).
type Adapter interface {
	Adapt(bundle string, a *indexio.ArtifactDetail, rt model.Runtime) ([]AdaptedFile, error)
}

// FakeAdapter returns canned payloads keyed by "path#key" (merge) or "path" (file).
// Used by every test in this slice so no test needs the real adapter.
type FakeAdapter struct{ payloads map[string]string }

func NewFakeAdapter(payloads map[string]string) *FakeAdapter {
	return &FakeAdapter{payloads: payloads}
}

func (f *FakeAdapter) Adapt(_ string, a *indexio.ArtifactDetail, rt model.Runtime) ([]AdaptedFile, error) {
	outs, ok := a.Outputs[rt]
	if !ok {
		return nil, fmt.Errorf("artifact %s/%s does not target %s", a.Type, a.Name, rt)
	}
	var res []AdaptedFile
	for _, o := range outs {
		key := o.Path
		if o.Key != "" {
			key = o.Path + "#" + o.Key
		}
		payload := f.payloads[key]
		if payload == "" {
			payload = fmt.Sprintf("# %s/%s for %s\n", a.Type, a.Name, rt)
		}
		res = append(res, AdaptedFile{
			Path: o.Path, Kind: o.Kind, Key: o.Key, Sentinel: o.Sentinel,
			Emulated: o.Emulated, Payload: payload,
		})
	}
	return res, nil
}
