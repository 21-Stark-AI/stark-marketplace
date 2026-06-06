// Package registry assembles the per-runtime adapter targets keyed by runtime.
//
// It lives in its own package (NOT `adapter`) to avoid an import cycle: every
// target (claude/codex/gemini) imports `adapter` for OutputFile/Finding/Target,
// so `adapter` itself cannot import the targets. registry sits above them all.
// Each target is independently versioned (spec §7.7) so a format fix to one
// runtime churns only that runtime's output.
package registry

import (
	"github.com/GetEvinced/stark-marketplace/engine/internal/adapter"
	"github.com/GetEvinced/stark-marketplace/engine/internal/adapter/claude"
	"github.com/GetEvinced/stark-marketplace/engine/internal/adapter/codex"
	"github.com/GetEvinced/stark-marketplace/engine/internal/adapter/gemini"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// All returns every runtime target keyed by runtime (claude + codex + gemini).
func All() map[model.Runtime]adapter.Target {
	return map[model.Runtime]adapter.Target{
		model.RuntimeClaude: claude.New(),
		model.RuntimeCodex:  codex.New(),
		model.RuntimeGemini: gemini.New(),
	}
}

// NonClaude returns the Codex + Gemini targets — the runtimes whose dist trees are
// built on `stark install` and never committed (spec §5.1).
func NonClaude() map[model.Runtime]adapter.Target {
	return map[model.Runtime]adapter.Target{
		model.RuntimeCodex:  codex.New(),
		model.RuntimeGemini: gemini.New(),
	}
}
