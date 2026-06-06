package main

import (
	"encoding/json"
	"io"
)

// Exit codes — spec §9.8. Do not renumber.
const (
	ExitOK              = 0
	ExitValidation      = 1
	ExitDrift           = 2
	ExitDigest          = 3
	ExitConflict        = 4
	ExitSchemaVersion   = 5
	ExitConsentDeclined = 6
)

// emitJSON writes a machine-readable envelope to w. Used by every command under --json.
func emitJSON(w io.Writer, command string, exit int, data map[string]any) {
	env := map[string]any{"command": command, "exit": exit}
	for k, v := range data {
		env[k] = v
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(env)
}
