package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEmitJSON(t *testing.T) {
	var buf bytes.Buffer
	emitJSON(&buf, "install", ExitOK, map[string]any{"bundle": "stark-gh"})
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["command"] != "install" || got["exit"].(float64) != 0 {
		t.Fatalf("envelope wrong: %v", got)
	}
}

func TestExitCodeConstants(t *testing.T) {
	// spec §9.8 contract — these values are load-bearing for CI + scripts.
	cases := map[int]int{
		ExitOK: 0, ExitValidation: 1, ExitDrift: 2, ExitDigest: 3,
		ExitConflict: 4, ExitSchemaVersion: 5, ExitConsentDeclined: 6,
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("exit code %d != %d", got, want)
		}
	}
}
