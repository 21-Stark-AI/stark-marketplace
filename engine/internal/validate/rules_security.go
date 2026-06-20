package validate

import (
	"path"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func checkSecurity(r *Result, where string, a *model.Artifact) {
	if a.Type != model.TypeMCP || a.MCP == nil {
		return
	}
	m := a.MCP
	// The command-allowlist + inline-eval/npx checks are the highest-trust surface
	// (spec §7.4) and must NOT depend on a self-declared transport label: scrutinize
	// whenever a command is present, regardless of transport. The schema permits
	// command/args on http too, so gating on transport=="stdio" would fail open.
	if m.Command != "" {
		base := path.Base(m.Command)
		if base != m.Command && strings.Contains(m.Command, "/") {
			r.Errorf(where, "mcp.command must be a bare allowlisted basename, got path %q", m.Command)
		}
		if !commandAllowlist[base] {
			r.Errorf(where, "mcp.command %q not on the allowlist (spec §7.4)", base)
		}
		for _, arg := range m.Args {
			// Match the forbidden flag whether it stands alone (`--eval CODE`) or
			// carries its value attached with `=` (`--eval=CODE`) — interpreters like
			// node honor the attached form, which an exact-token match would miss.
			flag := arg
			if i := strings.IndexByte(arg, '='); i >= 0 {
				flag = arg[:i]
			}
			if inlineEvalFlags[flag] {
				r.Errorf(where, "mcp.args contains inline-eval flag %q", arg)
			}
		}
		// Unpinned-npx: `npx` is the COMMAND, with the package as the first non-flag
		// arg (e.g. `command: npx`, `args: ["-y", "pkg@1.2.3"]`). Warn when that
		// package spec carries no @version/digest pin (spec §7.4).
		if base == "npx" {
			for _, arg := range m.Args {
				if strings.HasPrefix(arg, "-") {
					continue
				}
				if !strings.Contains(arg, "@") {
					r.Warnf(where, "unpinned npx package %q — pin a version/digest", arg)
				}
				break
			}
		}
	}
	// inline credential scan in args/url (defense-in-depth; env is structurally safe)
	for _, arg := range m.Args {
		scanInlineCred(r, where, arg)
	}
	scanInlineCred(r, where, m.URL)
}

func scanInlineCred(r *Result, where, s string) {
	low := strings.ToLower(s)
	// Patterns that warn ON THEIR OWN (a literal value follows the token).
	for _, p := range []string{"token=", "key=", "password=", "passwd=", "secret=", "apikey=", "--token=", "--password="} {
		if strings.Contains(low, p) {
			r.Warnf(where, "possible inline credential in %q — use secretRef", s)
			return
		}
	}
	// userinfo-style creds (e.g. https://user:pass@host) only matter when an `@` follows.
	for _, p := range []string{"--token", "--password", "://"} {
		if strings.Contains(low, p) && strings.Contains(low, "@") {
			r.Warnf(where, "possible inline credential in %q — use secretRef", s)
			return
		}
	}
}
