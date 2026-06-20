package installplan

import (
	"fmt"
	"sort"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/indexio"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// Step is one artifact to install, in DAG order.
type Step struct {
	Bundle   string
	Name     string
	Type     model.ArtifactType
	Files    []AdaptedFile
	Skipped  bool // true when the artifact does not target the requested runtime
	SkipNote string
}

// ConsentPayload is the human-facing summary for code-executing classes (spec §9.3).
type ConsentPayload struct {
	Required        bool     // true if any mcp/agent in the closure
	MCPCommands     []string // "name: command arg arg"
	AgentToolGrants []string // "name: tool, tool"
	ClosureRefs     []string // full resolved closure, transitive mcp/agent highlighted
}

// Plan is the full computed install for one root artifact/bundle on one runtime.
type Plan struct {
	Runtime model.Runtime
	Steps   []Step
	Consent ConsentPayload
	Skipped []string // bundle/name skipped (don't target runtime)
}

// node identifies an artifact within the catalog.
type node struct {
	bundle string
	name   string
	typ    model.ArtifactType
}

func (n node) ref() string { return n.bundle + "/" + n.name }

// resolveRef turns a Requirement.Ref ("name" or "bundle/name") into a node, looking up the
// type from the index. Same-bundle refs default to ownerBundle.
func resolveRef(idx *indexio.Index, ownerBundle string, req model.Requirement) (node, error) {
	bundle, name := ownerBundle, req.Ref
	for i := 0; i < len(req.Ref); i++ {
		if req.Ref[i] == '/' {
			bundle, name = req.Ref[:i], req.Ref[i+1:]
			break
		}
	}
	if e := idx.Find(bundle, name, req.Type); e == nil {
		return node{}, fmt.Errorf("unresolved dependency %s/%s (%s)", bundle, name, req.Type)
	}
	return node{bundle: bundle, name: name, typ: req.Type}, nil
}

// detailCache loads bundle details on demand.
type detailCache struct {
	dir   string
	cache map[string]*indexio.BundleDetail
}

func newCache(dir string) *detailCache {
	return &detailCache{dir: dir, cache: map[string]*indexio.BundleDetail{}}
}

func (c *detailCache) get(bundle string) (*indexio.BundleDetail, error) {
	if d, ok := c.cache[bundle]; ok {
		return d, nil
	}
	d, err := indexio.LoadBundleDetail(c.dir, bundle)
	if err != nil {
		return nil, err
	}
	c.cache[bundle] = d
	return d, nil
}

// topo returns nodes in dependency-first order (deps before dependents) via DFS post-order.
// Cycle detection errors. Deterministic: requires are visited in sorted ref order.
func topo(idx *indexio.Index, c *detailCache, root node) ([]node, error) {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var order []node
	var visit func(n node) error
	visit = func(n node) error {
		switch color[n.ref()+"|"+string(n.typ)] {
		case gray:
			return fmt.Errorf("dependency cycle at %s", n.ref())
		case black:
			return nil
		}
		color[n.ref()+"|"+string(n.typ)] = gray
		d, err := c.get(n.bundle)
		if err != nil {
			return err
		}
		ad := d.Artifact(n.name, n.typ)
		if ad == nil {
			return fmt.Errorf("artifact %s/%s (%s) not in detail", n.bundle, n.name, n.typ)
		}
		reqs := append([]model.Requirement(nil), ad.Requires...)
		sort.Slice(reqs, func(i, j int) bool { return reqs[i].Ref < reqs[j].Ref })
		for _, req := range reqs {
			dep, err := resolveRef(idx, n.bundle, req)
			if err != nil {
				return err
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		color[n.ref()+"|"+string(n.typ)] = black
		order = append(order, n)
		return nil
	}
	if err := visit(root); err != nil {
		return nil, err
	}
	return order, nil
}

// rootNodes returns the install roots: a single artifact, or every artifact in the bundle.
func rootNodes(c *detailCache, bundle, artifact string, typ model.ArtifactType) ([]node, error) {
	d, err := c.get(bundle)
	if err != nil {
		return nil, err
	}
	if artifact != "" {
		ad := d.Artifact(artifact, typ)
		if ad == nil {
			return nil, fmt.Errorf("artifact %s/%s (%s) not found", bundle, artifact, typ)
		}
		return []node{{bundle, artifact, typ}}, nil
	}
	var ns []node
	for _, a := range d.Artifacts {
		ns = append(ns, node{bundle, a.Name, a.Type})
	}
	sort.Slice(ns, func(i, j int) bool { return ns[i].name < ns[j].name })
	return ns, nil
}

// Compute builds the full plan: runtime subset + DAG order + consent.
func Compute(idx *indexio.Index, bundlesDir string, ad Adapter, bundle, artifact string,
	typ model.ArtifactType, rt model.Runtime) (*Plan, error) {
	c := newCache(bundlesDir)
	roots, err := rootNodes(c, bundle, artifact, typ)
	if err != nil {
		return nil, err
	}
	p := &Plan{Runtime: rt}
	seen := map[string]bool{}
	for _, root := range roots {
		order, err := topo(idx, c, root)
		if err != nil {
			return nil, err
		}
		for _, n := range order {
			id := n.ref() + "|" + string(n.typ)
			if seen[id] {
				continue
			}
			seen[id] = true
			d, _ := c.get(n.bundle)
			adet := d.Artifact(n.name, n.typ)
			lvl, ok := adet.Support[rt]
			if !ok || lvl == model.SupportUnsupported {
				p.Skipped = append(p.Skipped, n.ref())
				continue
			}
			files, err := ad.Adapt(n.bundle, adet, rt)
			if err != nil {
				return nil, err
			}
			p.Steps = append(p.Steps, Step{Bundle: n.bundle, Name: n.name, Type: n.typ, Files: files})
			addConsent(&p.Consent, n, adet)
		}
	}
	sort.Strings(p.Skipped)
	sort.Strings(p.Consent.ClosureRefs)
	sort.Strings(p.Consent.MCPCommands)
	sort.Strings(p.Consent.AgentToolGrants)
	return p, nil
}

// ClosureRefs returns the transitive dependency closure of an artifact (excluding itself),
// as sorted "bundle/name" strings — used by `stark info`.
func ClosureRefs(idx *indexio.Index, bundlesDir, bundle, artifact string, typ model.ArtifactType) ([]string, error) {
	c := newCache(bundlesDir)
	order, err := topo(idx, c, node{bundle, artifact, typ})
	if err != nil {
		return nil, err
	}
	var refs []string
	for _, n := range order {
		if n.bundle == bundle && n.name == artifact && n.typ == typ {
			continue
		}
		refs = append(refs, n.ref())
	}
	sort.Strings(refs)
	return refs, nil
}
