package validate

import (
	"fmt"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

type Finding struct {
	Where string
	Msg   string
}

type Result struct {
	Errors   []Finding
	Warnings []Finding
}

func (r *Result) Errorf(where, format string, a ...any) {
	r.Errors = append(r.Errors, Finding{where, fmt.Sprintf(format, a...)})
}
func (r *Result) Warnf(where, format string, a ...any) {
	r.Warnings = append(r.Warnings, Finding{where, fmt.Sprintf(format, a...)})
}
func (r *Result) HasErrors() bool { return len(r.Errors) > 0 }

// Catalog runs all rules over a loaded catalog.
func Catalog(cat *model.Catalog) *Result {
	r := &Result{}
	for _, b := range cat.Bundles {
		checkSlug(r, "bundle:"+b.Name, b.Name)
		for _, a := range b.Artifacts {
			where := b.Name + "/" + string(a.Type) + "/" + a.Name
			if a.Raw == nil {
				r.Errorf(where, "frontmatter could not be parsed for schema validation")
			} else if err := ValidateSchema(string(a.Type), a.Raw); err != nil {
				r.Errorf(where, "schema: %v", err)
			}
			checkSlug(r, where, a.Name)
			checkRuntimesNarrowing(r, where, a, b)
			checkSecurity(r, where, a)
			checkFences(r, where, a)
			checkCapability(r, where, a)
		}
		checkOutputUniqueness(r, b)
	}
	return r
}
