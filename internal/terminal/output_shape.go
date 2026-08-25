package terminal

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const defaultExecOutputBytes = 16 * 1024

func shapeOutput(content []byte, shape OutputShape) (string, bool) {
	if shape.Grep != "" {
		content = grepOutput(content, shape.Grep)
	}
	maximum := shape.MaxBytes
	if maximum <= 0 {
		maximum = defaultExecOutputBytes
	}
	if len(content) <= maximum {
		return string(content), false
	}
	strategy := strings.TrimSpace(shape.Strategy)
	if strategy == "" {
		strategy = "head_tail"
	}
	if strategy == "tail" {
		return elidedTail(content, maximum), true
	}
	return elidedHeadTail(content, maximum), true
}

func grepOutput(content []byte, pattern string) []byte {
	lines := strings.Split(string(content), "\n")
	matches := make([]string, 0)
	for _, line := range lines {
		if strings.Contains(line, pattern) {
			matches = append(matches, line)
		}
	}
	if len(matches) == 0 {
		return []byte(fmt.Sprintf("0 matches of %d lines", len(lines)))
	}
	return []byte(strings.Join(matches, "\n"))
}

func elidedTail(content []byte, maximum int) string {
	for retained := maximum; retained >= 0; retained-- {
		omitted := len(content) - retained
		marker := fmt.Sprintf("⟨%d bytes elided⟩\n", omitted)
		if len(marker)+retained <= maximum {
			tail := trimPartialLeadingRune(content[len(content)-retained:])
			return marker + string(tail)
		}
	}
	return ""
}

func elidedHeadTail(content []byte, maximum int) string {
	for retained := maximum; retained >= 0; retained-- {
		omitted := len(content) - retained
		marker := fmt.Sprintf("\n⟨%d bytes elided⟩\n", omitted)
		if len(marker)+retained > maximum {
			continue
		}
		headBytes := retained / 2
		tailBytes := retained - headBytes
		head := trimPartialTrailingRune(content[:headBytes])
		tail := trimPartialLeadingRune(content[len(content)-tailBytes:])
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
