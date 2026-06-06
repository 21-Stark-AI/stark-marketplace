package indexio

import "fmt"

// SchemaVersionMin/Max is the index.schemaVersion range this stark binary supports.
// N-1 compat per spec §7.5: bump Max on additive-within-version; keep Min one behind
// when a genuine break ships so older catalogs still install.
const (
	SchemaVersionMin = 1
	SchemaVersionMax = 1
)

// AssertSchemaVersion returns a non-nil error (exit code 5 at the CLI boundary) when
// the index schemaVersion is outside the supported range.
func AssertSchemaVersion(v int) error {
	if v < SchemaVersionMin || v > SchemaVersionMax {
		return fmt.Errorf("index schemaVersion %d unsupported (this stark supports %d..%d); run `stark self-update`",
			v, SchemaVersionMin, SchemaVersionMax)
	}
	return nil
}
