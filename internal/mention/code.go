package mention

import "strings"

// span is a half-open byte range of text.
type span struct {
	start int
	end   int
}

// codeRegions returns the sorted, non-overlapping byte ranges of text covered
// by fenced code blocks and inline code spans (Business Rule 3).
//
// Fences follow CommonMark: a line indented at most three spaces opening with
// three or more backticks or tildes; the block closes at a line holding a
// fence of the same character at least as long, or runs to the end of text.
// Inline spans outside fences open with a run of N backticks and close at the
// next run of exactly N backticks; an unmatched run is literal text.
func codeRegions(text string) []span {
	if !strings.ContainsAny(text, "`~") {
		return nil
	}
	var regions []span
	fence := byte(0)
	fenceLength := 0
	fenceStart := 0
	proseStart := 0
	for lineStart := 0; lineStart < len(text); {
		lineEnd := strings.IndexByte(text[lineStart:], '\n')
		next := len(text)
		if lineEnd >= 0 {
			next = lineStart + lineEnd + 1
		}
		marker, length, isFence := fenceMarker(text[lineStart:next])
		switch {
		case fence == 0 && isFence:
			regions = appendInlineSpans(regions, text, proseStart, lineStart)
			fence, fenceLength, fenceStart = marker, length, lineStart
		case fence != 0 && isFence && marker == fence && length >= fenceLength &&
			strings.TrimSpace(text[lineStart:next])[length:] == "":
			regions = append(regions, span{start: fenceStart, end: next})
			fence, proseStart = 0, next
		}
		lineStart = next
	}
	if fence != 0 {
		return append(regions, span{start: fenceStart, end: len(text)})
	}
	return appendInlineSpans(regions, text, proseStart, len(text))
}

// fenceMarker reports whether line opens or closes a fence, with its rune and
// run length.
func fenceMarker(line string) (byte, int, bool) {
	indent := 0
	for indent < len(line) && indent < 4 && line[indent] == ' ' {
		indent++
	}
	if indent > 3 || indent >= len(line) {
		return 0, 0, false
	}
	marker := line[indent]
	if marker != '`' && marker != '~' {
		return 0, 0, false
	}
	length := 0
	for indent+length < len(line) && line[indent+length] == marker {
		length++
	}
	if length < 3 {
		return 0, 0, false
	}
	if marker == '`' && strings.Contains(line[indent+length:], "`") {
		return 0, 0, false
	}
	return marker, length, true
}

// appendInlineSpans appends the inline code spans found in text[from:to].
func appendInlineSpans(regions []span, text string, from int, to int) []span {
	for offset := from; offset < to; {
		open := strings.IndexByte(text[offset:to], '`')
		if open < 0 {
			break
		}
		start := offset + open
		length := backtickRun(text[start:to])
		closing := findBacktickRun(text[start+length:to], length)
		if closing < 0 {
			offset = start + length
			continue
		}
		end := start + length + closing + length
		regions = append(regions, span{start: start, end: end})
		offset = end
	}
	return regions
}

func backtickRun(text string) int {
	length := 0
	for length < len(text) && text[length] == '`' {
		length++
	}
	return length
}

// findBacktickRun returns the offset of the first run of exactly length
// backticks in text, or -1.
func findBacktickRun(text string, length int) int {
	for offset := 0; offset < len(text); {
		open := strings.IndexByte(text[offset:], '`')
		if open < 0 {
			return -1
		}
		start := offset + open
		run := backtickRun(text[start:])
		if run == length {
			return start
		}
		offset = start + run
	}
	return -1
}
