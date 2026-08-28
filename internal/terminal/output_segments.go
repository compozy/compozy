package terminal

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const MaximumOutputTailBytes = 16 * 1024

func RedactedInputMarker(characters int) OutputSegment {
	return OutputSegment{Kind: OutputSegmentRedactedInput, Characters: max(characters, 0)}
}

func RenderOutputSegment(segment OutputSegment) string {
	switch segment.Kind {
	case OutputSegmentBytes:
		return segment.Text
	case OutputSegmentRedactedInput:
		return fmt.Sprintf("hidden input · %d characters", max(segment.Characters, 0))
	default:
		return ""
	}
}

func RenderOutputSegments(segments []OutputSegment) string {
	var rendered strings.Builder
	for _, segment := range segments {
		rendered.WriteString(RenderOutputSegment(segment))
	}
	return rendered.String()
}

func OutputTailFromBytes(output []byte) []OutputSegment {
	if len(output) == 0 {
		return nil
	}
	return BoundedOutputTail([]OutputSegment{{Kind: OutputSegmentBytes, Text: string(output)}})
}

func BoundedOutputTail(segments []OutputSegment) []OutputSegment {
	retained := make([]OutputSegment, 0, len(segments))
	remaining := MaximumOutputTailBytes
	for index := len(segments) - 1; index >= 0 && remaining > 0; index-- {
		segment := segments[index]
		rendered := RenderOutputSegment(segment)
		if len(rendered) > remaining {
			if segment.Kind != OutputSegmentBytes {
				break
			}
			segment.Text = validOutputTailSuffix(segment.Text, remaining)
			rendered = segment.Text
		}
		if rendered == "" {
			continue
		}
		remaining -= len(rendered)
		retained = append(retained, segment)
	}
	for left, right := 0, len(retained)-1; left < right; left, right = left+1, right-1 {
		retained[left], retained[right] = retained[right], retained[left]
	}
	return retained
}

func validOutputTailSuffix(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	bytes := []byte(value[len(value)-maximum:])
	for len(bytes) > 0 && !utf8.Valid(bytes) {
		bytes = bytes[1:]
	}
	return string(bytes)
}

func cloneOutputSegments(segments []OutputSegment) []OutputSegment {
	return append([]OutputSegment(nil), segments...)
}
