package terminal

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const defaultExecOutputBytes = 16 * 1024

func shapeOutput(content []byte, shape OutputShape) (string, bool, error) {
	if shape.Grep != "" {
		var err error
		content, err = grepOutput(content, shape.Grep)
		if err != nil {
			return "", false, err
		}
	}
	maximum := shape.MaxBytes
	if maximum <= 0 {
		maximum = defaultExecOutputBytes
	}
	if len(content) <= maximum {
		return string(content), false, nil
	}
	strategy := strings.TrimSpace(shape.Strategy)
	if strategy == "" {
		strategy = "head_tail"
	}
	switch strategy {
	case terminalViewTail:
		return elidedTail(content, maximum), true, nil
	case "head_tail":
		return elidedHeadTail(content, maximum), true, nil
	default:
		return "", false, fmt.Errorf("terminal output strategy must be tail or head_tail: %w", ErrUnsupported)
	}
}

func grepOutput(content []byte, pattern string) ([]byte, error) {
	expression, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("terminal grep pattern is invalid: %w", errors.Join(ErrUnsupported, err))
	}
	lines := strings.Split(string(content), "\n")
	matches := make([]string, 0)
	for _, line := range lines {
		if expression.MatchString(line) {
			matches = append(matches, line)
		}
	}
	if len(matches) == 0 {
		return []byte(fmt.Sprintf("0 matches of %d lines", len(lines))), nil
	}
	return []byte(strings.Join(matches, "\n")), nil
}

func elidedTail(content []byte, maximum int) string {
	for retained := maximum; retained >= 0; retained-- {
		tail := trimPartialLeadingRune(content[len(content)-retained:])
		omitted := len(content) - len(tail)
		marker := fmt.Sprintf("⟨%d bytes elided⟩\n", omitted)
		if len(marker)+len(tail) <= maximum {
			return marker + string(tail)
		}
	}
	return ""
}

func elidedHeadTail(content []byte, maximum int) string {
	for retained := maximum; retained >= 0; retained-- {
		headBytes := retained / 2
		tailBytes := retained - headBytes
		head := trimPartialTrailingRune(content[:headBytes])
		tail := trimPartialLeadingRune(content[len(content)-tailBytes:])
		omitted := len(content) - len(head) - len(tail)
		marker := fmt.Sprintf("\n⟨%d bytes elided⟩\n", omitted)
		if len(marker)+len(head)+len(tail) > maximum {
			continue
		}
		return string(head) + marker + string(tail)
	}
	return ""
}

func trimPartialTrailingRune(content []byte) []byte {
	for len(content) > 0 && !utf8.Valid(content) {
		content = content[:len(content)-1]
	}
	return content
}
