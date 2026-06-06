package validate

import (
	"github.com/GetEvinced/stark-marketplace/engine/internal/fence"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// checkFences strips the body for each targeted runtime; any parse error is reported.
func checkFences(r *Result, where string, a *model.Artifact) {
	for _, rt := range a.Runtimes {
		if _, err := fence.Strip(a.Body, rt, a.Runtimes); err != nil {
			r.Errorf(where, "fence error: %v", err)
		}
	}
}
