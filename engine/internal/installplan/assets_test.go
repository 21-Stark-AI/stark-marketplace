package installplan

import (
	"errors"
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

// Assets are code the operator is consenting to install, but they belong to no
// artifact — they must not inflate the consent closure.
func TestAssetsStepAddsNoConsent(t *testing.T) {
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
		t.Fatalf("assets changed the consent closure: %v vs %v",
			withAssets.Consent.ClosureRefs, plain.Consent.ClosureRefs)
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
