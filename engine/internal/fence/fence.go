package fence

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

var (
	openRe  = regexp.MustCompile(`^<!--\s*runtime:\s*(!?)([a-z0-9]+(?:\s*,\s*[a-z0-9]+)*)\s*-->\s*$`)
	closeRe = regexp.MustCompile(`^<!--\s*/runtime\s*-->\s*$`)
)

// Strip removes fenced regions not applicable to `target`. `targeted` is the artifact's
// full runtime set (used to validate the `except` form and unknown tokens).
func Strip(body string, target model.Runtime, targeted []model.Runtime) (string, error) {
	lines := strings.Split(body, "\n")
	var out []string
	inFence := false
	keep := true
	for i, ln := range lines {
		switch {
		case openRe.MatchString(ln):
			if inFence {
				return "", fmt.Errorf("line %d: nested runtime fence", i+1)
			}
			neg, list := parseOpen(ln)
			runtimes, err := resolveTokens(list, targeted)
			if err != nil {
				return "", fmt.Errorf("line %d: %w", i+1, err)
			}
			inFence = true
			match := contains(runtimes, target)
			if neg {
				keep = !match
			} else {
				keep = match
			}
		case closeRe.MatchString(ln):
			if !inFence {
				return "", fmt.Errorf("line %d: unmatched /runtime", i+1)
			}
			inFence = false
			keep = true
		default:
			if !inFence || keep {
				out = append(out, ln)
			}
		}
	}
	if inFence {
		return "", fmt.Errorf("unterminated runtime fence")
	}
	return strings.Join(out, "\n"), nil
}

func parseOpen(ln string) (neg bool, list string) {
	m := openRe.FindStringSubmatch(ln)
	return m[1] == "!", m[2]
}

func resolveTokens(list string, targeted []model.Runtime) ([]model.Runtime, error) {
	var rs []model.Runtime
	for _, tok := range strings.Split(list, ",") {
		tok = strings.TrimSpace(tok)
		r, err := model.ParseRuntime(tok)
		if err != nil {
			return nil, err
		}
		if !contains(targeted, r) {
			return nil, fmt.Errorf("fence runtime %q not in artifact's targeted set", r)
		}
		rs = append(rs, r)
	}
	return rs, nil
}

func contains(rs []model.Runtime, r model.Runtime) bool {
	for _, x := range rs {
		if x == r {
			return true
		}
	}
	return false
}
