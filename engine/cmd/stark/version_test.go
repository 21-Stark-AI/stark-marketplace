package main

import (
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/indexio"
)

func TestVersionStringIncludesSchemaRange(t *testing.T) {
	s := versionString()
	if !strings.Contains(s, "schemaVersion") {
		t.Fatalf("version must report schemaVersion range: %s", s)
	}
}

func TestSelfUpdateRejectsUnsupportedIndex(t *testing.T) {
	// an index newer than we support → exit 5 path
	code := checkIndexSupported(indexio.SchemaVersionMax + 1)
	if code != ExitSchemaVersion {
		t.Fatalf("want exit 5, got %d", code)
	}
	if checkIndexSupported(indexio.SchemaVersionMax) != ExitOK {
		t.Fatal("supported version must be ok")
	}
}
