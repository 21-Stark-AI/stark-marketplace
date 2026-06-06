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
	// codeRe matches a markdown fenced-code-block delimiter (``` or ~~~, 3+).
	codeRe = regexp.MustCompile("^\\s*(```+|~~~+)")
)

// Strip removes fenced regions not applicable to `target`. `targeted` is the artifact's
// full runtime set (used to validate the `except` form and unknown tokens).
//
// Error taxonomy (spec §4.2): unterminated fence, nested fence, unknown runtime token,
// runtime not in the targeted set, a runtime fence appearing inside a fenced code block,
// and an empty fence section.
func Strip(body string, target model.Runtime, targeted []model.Runtime) (string, error) {
	lines := strings.Split(body, "\n")
	var out []string
	inFence := false
	keep := true
	inCode := false
	sectionHasContent := false
	for i, ln := range lines {
		isCodeFence := codeRe.MatchString(ln)
		isOpen := openRe.MatchString(ln)
		isClose := closeRe.MatchString(ln)

		// A runtime fence marker inside a fenced code block is a validation error
		// (spec §4.2). Detect before toggling code state so a marker on a code line
		// is caught rather than silently treated as a fence.
		if inCode && (isOpen || isClose) {
			return "", fmt.Errorf("line %d: runtime fence inside a fenced code block", i+1)
		}

		switch {
		case isCodeFence:
			inCode = !inCode
			if inFence {
				sectionHasContent = true
				if keep {
					out = append(out, ln)
				}
			} else {
				out = append(out, ln)
			}
		case isOpen:
			if inFence {
				return "", fmt.Errorf("line %d: nested runtime fence", i+1)
			}
			neg, list := parseOpen(ln)
			runtimes, err := resolveTokens(list, targeted)
			if err != nil {
				return "", fmt.Errorf("line %d: %w", i+1, err)
			}
			inFence = true
			sectionHasContent = false
			match := contains(runtimes, target)
			keep = match
			if neg {
				keep = !match
			}
		case isClose:
			if !inFence {
				return "", fmt.Errorf("line %d: unmatched /runtime", i+1)
			}
			if !sectionHasContent {
				return "", fmt.Errorf("line %d: empty runtime fence section", i+1)
			}
			inFence = false
			keep = true
		default:
			if inFence {
				if strings.TrimSpace(ln) != "" {
					sectionHasContent = true
				}
				if keep {
					out = append(out, ln)
				}
			} else {
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
