package validate

import (
	"regexp"
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// LintBodies runs an informational content scan over skill/command/agent/prompt bodies.
// It NEVER produces errors (spec §7.4 body lint is informational/warning); every finding lands
// in Result.Warnings. It is intentionally separate from Catalog(): content lint is advisory and
// surfaced in PR output, not a fail-closed gate.
func LintBodies(cat *model.Catalog) *Result {
	r := &Result{}
	for _, b := range cat.Bundles {
		for _, a := range b.Artifacts {
			if !hasBody(a.Type) {
				continue
			}
			where := b.Name + "/" + string(a.Type) + "/" + a.Name
			scanBody(r, where, a.Body)
		}
	}
	return r
}

// hasBody reports whether an artifact type carries an instruction body (MCP has none).
func hasBody(t model.ArtifactType) bool {
	switch t {
	case model.TypeSkill, model.TypeCommand, model.TypeAgent, model.TypePrompt:
		return true
	default:
		return false
	}
}

var lintPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	// Match a download piped into an interpreter, including a privilege-elevated or
	// env-wrapped one: `curl … | sh`, `curl … | sudo bash`, `curl … | env FOO=1 sh`.
	{"curl-pipe-shell", regexp.MustCompile(`(?i)\b(curl|wget)\b[^\n|]*\|\s*(sudo\s+)?(env\s+\S+\s+)*(sh|bash|zsh|dash|ksh)\b`)},
	{"secret-file-read", regexp.MustCompile(`(?i)(\.env\b|\.private\b|\.aws/credentials|\.ssh/id_|/credentials\b|secrets?\.(json|ya?ml|toml))`)},
	{"prompt-injection", regexp.MustCompile(`(?i)(ignore|disregard|forget)\s+(all\s+)?(the\s+)?(previous|prior|above|earlier)\s+instructions`)},
}

// base64Blob matches a long contiguous base64-ish run (>=80 chars) — a common vector for
// hiding an executable payload inside an instruction body.
var base64Blob = regexp.MustCompile(`[A-Za-z0-9+/]{80,}={0,2}`)

func scanBody(r *Result, where, body string) {
	for _, p := range lintPatterns {
		if p.re.MatchString(body) {
			r.Warnf(where, "suspicious pattern [%s] in body", p.name)
		}
	}
	if base64Blob.MatchString(stripCodeWords(body)) {
		r.Warnf(where, "suspicious pattern [base64-blob] in body (>=80 char run)")
	}
}

// stripCodeWords removes ordinary long-but-harmless tokens (URLs) before the base64 heuristic
// so plain links don't trip it.
func stripCodeWords(body string) string {
	return strings.NewReplacer("https://", " ", "http://", " ").Replace(body)
}
