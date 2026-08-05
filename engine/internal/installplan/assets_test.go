package installplan

import (
	"errors"
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/model"
)

// assetAdapter is a FakeAdapter that also provides bundle assets.
type assetAdapter struct {
	*FakeAdapter
	calls []string
	err   error
}

func (a *assetAdapter) BundleAssets(bundle string, rt model.Runtime) ([]AdaptedFile, error) {
	a.calls = append(a.calls, bundle+"@"+string(rt))
	if a.err != nil {
		return nil, a.err
	}
	return []AdaptedFile{{Path: ".agents/stark/" + bundle + "/tools/x.ts", Kind: "file", Payload: "x\n"}}, nil
}

func TestComputeWithoutAssetProviderInstallsArtifactsOnly(t *testing.T) {
	idx, bdir := loadFx(t)
	p, err := Compute(idx, bdir, NewFakeAdapter(nil), "rev", "", model.TypeCommand, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range p.Steps {
		if s.Name == AssetsStepName {
			t.Fatalf("plain adapter must not produce an assets step: %+v", s)
		}
	}
}

// The assets step must come FIRST so a bundle's own artifacts win on any path
// collision — the precedence `stark build` applies to dist/claude.
func TestComputePrependsAssetsStep(t *testing.T) {
	idx, bdir := loadFx(t)
	ad := &assetAdapter{FakeAdapter: NewFakeAdapter(nil)}
	p, err := Compute(idx, bdir, ad, "rev", "", model.TypeCommand, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Steps) == 0 || p.Steps[0].Name != AssetsStepName {
		t.Fatalf("first step must be assets, got %+v", p.Steps)
	}
	if p.Steps[0].Files[0].Kind != "file" {
		t.Fatalf("assets must install as whole files: %+v", p.Steps[0].Files)
	}
	for _, s := range p.Steps[1:] {
		if s.Name == AssetsStepName {
			t.Fatal("assets emitted more than once for one bundle")
		}
	}
}

// Assets belong to no artifact, so they must not inflate the artifact consent CLOSURE
// (ClosureRefs) — but because they carry executable code the skills run, they DO flag
// consent via the dedicated AssetExec channel (see TestExecutableAssetsRequireConsent).
func TestAssetsStepAddsNoClosureRefs(t *testing.T) {
	idx, bdir := loadFx(t)
	plain, err := Compute(idx, bdir, NewFakeAdapter(nil), "rev", "", model.TypeCommand, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	withAssets, err := Compute(idx, bdir, &assetAdapter{FakeAdapter: NewFakeAdapter(nil)},
		"rev", "", model.TypeCommand, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	if len(withAssets.Consent.ClosureRefs) != len(plain.Consent.ClosureRefs) {
		t.Fatalf("assets changed the artifact consent closure: %v vs %v",
			withAssets.Consent.ClosureRefs, plain.Consent.ClosureRefs)
	}
}

// Vendored assets include executable code (tools/*.ts, scripts/*.sh) the installed
// skills shell out to — installing them is a code-execution surface and must sit inside
// the §9.3 consent gate even when the bundle ships no mcp/agent. The fixture asset is a
// .ts, so consent must be required and the bundle named in AssetExec.
func TestExecutableAssetsRequireConsent(t *testing.T) {
	idx, bdir := loadFx(t)
	ad := &assetAdapter{FakeAdapter: NewFakeAdapter(nil)}
	// Install the skill alone so the ONLY consent trigger is the executable asset,
	// not a transitive mcp/agent.
	p, err := Compute(idx, bdir, ad, "rev", "session", model.TypeSkill, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Consent.Required {
		t.Fatal("a bundle installing executable vendored assets must require consent")
	}
	if len(p.Consent.AssetExec) == 0 || !strings.Contains(p.Consent.AssetExec[0], "rev") {
		t.Fatalf("consent must name the bundle's executable assets: %v", p.Consent.AssetExec)
	}
}

// An MCP is a config.toml merge that references none of the vendored tools/standards,
// so installing ONLY an mcp must not drag in the bundle's asset tree (nor its consent).
func TestMCPOnlyInstallSkipsAssets(t *testing.T) {
	idx, bdir := loadFx(t)
	ad := &assetAdapter{FakeAdapter: NewFakeAdapter(nil)}
	p, err := Compute(idx, bdir, ad, "rev", "bq", model.TypeMCP, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range p.Steps {
		if s.Name == AssetsStepName {
			t.Fatalf("mcp-only install must not vendor assets: %+v", s)
		}
	}
	if len(ad.calls) != 0 {
		t.Fatalf("mcp-only install must not even request assets: %v", ad.calls)
	}
}

func TestBundleAssetsErrorFailsThePlan(t *testing.T) {
	idx, bdir := loadFx(t)
	ad := &assetAdapter{FakeAdapter: NewFakeAdapter(nil), err: errors.New("vendor dir unreadable")}
	if _, err := Compute(idx, bdir, ad, "rev", "", model.TypeCommand, model.RuntimeCodex); err == nil {
		t.Fatal("a failed asset read must fail the plan, not install a broken tree")
	}
}

// A bundle whose every requested artifact is skipped for this runtime installs
// nothing — including no assets. `session` is unsupported on gemini.
func TestNoAssetsWhenEveryStepIsSkipped(t *testing.T) {
	idx, bdir := loadFx(t)
	ad := &assetAdapter{FakeAdapter: NewFakeAdapter(nil)}
	p, err := Compute(idx, bdir, ad, "rev", "session", model.TypeSkill, model.RuntimeGemini)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Steps) != 0 {
		t.Fatalf("session is unsupported on gemini; want no steps, got %+v", p.Steps)
	}
	if len(ad.calls) != 0 {
		t.Fatalf("nothing was installed, so no assets should have been requested: %v", ad.calls)
	}
}
