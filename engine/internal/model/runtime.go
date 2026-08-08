package model

import "fmt"

type Runtime string

const (
	RuntimeClaude Runtime = "claude"
	RuntimeCodex  Runtime = "codex"
	RuntimeGemini Runtime = "gemini"
)

// AllRuntimes returns the canonical runtime set in deterministic order.
func AllRuntimes() []Runtime { return []Runtime{RuntimeClaude, RuntimeCodex, RuntimeGemini} }

// ContainsRuntime reports whether set includes rt.
func ContainsRuntime(set []Runtime, rt Runtime) bool {
	for _, r := range set {
		if r == rt {
			return true
		}
	}
	return false
}

func ParseRuntime(s string) (Runtime, error) {
	for _, r := range AllRuntimes() {
		if string(r) == s {
			return r, nil
		}
	}
	return "", fmt.Errorf("unknown runtime %q", s)
}

type ArtifactType string

const (
	TypeSkill   ArtifactType = "skill"
	TypePrompt  ArtifactType = "prompt"
	TypeCommand ArtifactType = "command"
	TypeAgent   ArtifactType = "agent"
	TypeMCP     ArtifactType = "mcp"
)

func AllArtifactTypes() []ArtifactType {
	return []ArtifactType{TypeSkill, TypePrompt, TypeCommand, TypeAgent, TypeMCP}
}

type SupportLevel string

const (
	SupportNative      SupportLevel = "native"
	SupportEmulated    SupportLevel = "emulated"
	SupportUnsupported SupportLevel = "unsupported"
)

type Maturity string

const (
	MaturityExperimental Maturity = "experimental"
	MaturityBeta         Maturity = "beta"
	MaturityStable       Maturity = "stable"
	MaturityDeprecated   Maturity = "deprecated"
)
