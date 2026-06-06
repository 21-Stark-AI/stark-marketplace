package validate

import (
	"github.com/GetEvinced/stark-marketplace/engine/internal/adapter/capability"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// checkCapability enforces the §6 matrix: warn on emulated targets (counted +
// surfaced), error on unsupported ones (spec §7.4).
func checkCapability(r *Result, where string, a *model.Artifact) {
	for _, rt := range a.Runtimes {
		switch capability.Level(a.Type, rt) {
		case model.SupportEmulated:
			r.Warnf(where, "%s on %s is emulated — verify fidelity (§6.1)", a.Type, rt)
		case model.SupportUnsupported:
			r.Errorf(where, "%s on %s is unsupported", a.Type, rt)
		}
	}
}
