package bridgesdk

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const chunkFenceClose = "\n```"

type chunkFenceLinePhase uint8

const (
	chunkFenceLineLeading chunkFenceLinePhase = iota
	chunkFenceLineTicks
	chunkFenceLineOpening
	chunkFenceLineLanguage
	chunkFenceLineContent
)

type chunkFenceState struct {
	open       bool
	language   string
	linePhase  chunkFenceLinePhase
	lineTicks  int
	markerLine bool
}

// ChunkMessage splits text within limit while preserving fenced code blocks.
// Limits too small for decorations fall back to undecorated lossless chunks.
func ChunkMessage(text string, limit int, lenFn func(string) int) []string {
	if lenFn == nil {
		lenFn = messageRuneLen
	}
	if limit <= 0 || lenFn(text) <= limit {
		return []string{text}
	}

	if chunks, ok := decoratedMessageChunks(text, limit, lenFn); ok {
		return chunks
	}
	return splitUndecoratedMessage(text, limit, lenFn)
}

func decoratedMessageChunks(text string, limit int, lenFn func(string) int) ([]string, bool) {
	indicatorReserve := lenFn(chunkIndicator(1, 2))
	maxAttempts := decimalLen(max(messageRuneLen(text), 2)) + 2
	for range maxAttempts {
		bodies, ok := splitChunkBodies(text, limit, indicatorReserve, lenFn)
		if !ok {
			return nil, false
		}
		if len(bodies) <= 1 {
			return nil, false
		}
		nextReserve := lenFn(chunkIndicator(len(bodies), len(bodies)))
		if nextReserve <= indicatorReserve {
			chunks := appendChunkIndicators(bodies)
			if chunksWithinLimit(chunks, limit, lenFn) {
				return chunks, true
			}
			return nil, false
		}
		indicatorReserve = nextReserve
	}

	return nil, false
}

// UTF16Len returns the number of UTF-16 code units used by value.
func UTF16Len(value string) int {
	length := 0
	for _, current := range value {
		if current > 0xffff {
			length += 2
		} else {
			length++
		}
	}
	return length
}

func splitChunkBodies(
	text string,
	limit int,
	indicatorReserve int,
	lenFn func(string) int,
) ([]string, bool) {
	remaining := text
	chunks := make([]string, 0, 2)
	state := chunkFenceState{linePhase: chunkFenceLineLeading}

	for remaining != "" {
		prefix := ""
		if state.open {
			prefix = "```" + state.language + "\n"
		}

		bodyLimit := limit - indicatorReserve - lenFn(prefix) - lenFn(chunkFenceClose)
		if bodyLimit < 1 {
			return nil, false
		}
		bodyEnd := longestChunkPrefix(remaining, bodyLimit, lenFn)
		if bodyEnd <= 0 {
			return nil, false
		}
		if bodyEnd < len(remaining) {
			bodyEnd = naturalChunkBoundary(remaining[:bodyEnd])
		}

		body := remaining[:bodyEnd]
		nextState := scanChunkFenceState(body, state)
		if bodyEnd < len(remaining) && nextState.boundaryUnsafe() {
			bodyEnd = previousChunkLineBoundary(body)
			if bodyEnd <= 0 {
				return nil, false
			}
			body = remaining[:bodyEnd]
			nextState = scanChunkFenceState(body, state)
			if nextState.boundaryUnsafe() {
				return nil, false
			}
		}

		chunk := prefix + body
		if nextState.open {
			chunk += chunkFenceClose
		}
		if lenFn(chunk)+indicatorReserve > limit {
			return nil, false
		}
		chunks = append(chunks, chunk)
		remaining = remaining[bodyEnd:]
		state = nextState
	}

	return chunks, true
}

func longestChunkPrefix(text string, limit int, lenFn func(string) int) int {
	runeEnds := chunkRuneEnds(text)
	low := 0
	high := len(runeEnds)
	for low < high {
		mid := low + (high-low+1)/2
		if lenFn(text[:runeEnds[mid-1]]) <= limit {
			low = mid
		} else {
			high = mid - 1
		}
	}
	if low == 0 {
		return 0
	}
	return runeEnds[low-1]
}

func naturalChunkBoundary(candidate string) int {
	threshold := max(utf8.RuneCountInString(candidate)/2, 1)
	lastNewlineEnd := -1
	lastWhitespaceEnd := -1
	runeIndex := 0
	for offset := 0; offset < len(candidate); {
		current, size := utf8.DecodeRuneInString(candidate[offset:])
		runeIndex++
		end := offset + size
		if runeIndex >= threshold && current == '\n' {
			lastNewlineEnd = end
		}
		if unicode.IsSpace(current) {
			if runeIndex >= threshold {
				lastWhitespaceEnd = end
			}
		}
		offset = end
	}
	if lastNewlineEnd > 0 {
		return lastNewlineEnd
	}
	if lastWhitespaceEnd > 0 {
		return lastWhitespaceEnd
	}
	return len(candidate)
}

func scanChunkFenceState(body string, state chunkFenceState) chunkFenceState {
	for _, current := range body {
		if current == '\n' {
			state.linePhase = chunkFenceLineLeading
			state.lineTicks = 0
			state.markerLine = false
			continue
		}
		state.consumeLineRune(current)
	}
	return state
}

func (s *chunkFenceState) consumeLineRune(current rune) {
	switch s.linePhase {
	case chunkFenceLineLeading:
		s.consumeLeadingRune(current)
	case chunkFenceLineTicks:
		s.consumeFenceTick(current)
	case chunkFenceLineOpening:
		if current == '`' || unicode.IsSpace(current) {
			return
		}
		s.language = string(current)
		s.linePhase = chunkFenceLineLanguage
	case chunkFenceLineLanguage:
		if unicode.IsSpace(current) {
			s.linePhase = chunkFenceLineContent
			return
		}
		s.language += string(current)
	case chunkFenceLineContent:
		return
	}
}

func (s *chunkFenceState) consumeLeadingRune(current rune) {
	switch current {
	case ' ', '\t', '\r':
		return
	case '`':
		s.linePhase = chunkFenceLineTicks
		s.lineTicks = 1
	default:
		s.linePhase = chunkFenceLineContent
	}
}

func (s *chunkFenceState) consumeFenceTick(current rune) {
	if current != '`' {
		s.linePhase = chunkFenceLineContent
		s.lineTicks = 0
		return
	}
	s.lineTicks++
	if s.lineTicks < 3 {
		return
	}
	s.markerLine = true
	if s.open {
		s.open = false
		s.language = ""
		s.linePhase = chunkFenceLineContent
		return
	}
	s.open = true
	s.language = ""
	s.linePhase = chunkFenceLineOpening
}

func (s chunkFenceState) boundaryUnsafe() bool {
	return s.markerLine || s.linePhase == chunkFenceLineTicks
}

func previousChunkLineBoundary(body string) int {
	lineBreak := strings.LastIndexByte(body, '\n')
	if lineBreak < 0 {
		return 0
	}
	return lineBreak + 1
}

func splitUndecoratedMessage(text string, limit int, lenFn func(string) int) []string {
	remaining := text
	chunks := make([]string, 0, 2)
	for remaining != "" {
		bodyEnd := longestChunkPrefix(remaining, limit, lenFn)
		if bodyEnd <= 0 {
			_, bodyEnd = utf8.DecodeRuneInString(remaining)
		}
		if bodyEnd < len(remaining) {
			bodyEnd = naturalChunkBoundary(remaining[:bodyEnd])
		}
		chunks = append(chunks, remaining[:bodyEnd])
		remaining = remaining[bodyEnd:]
	}
	return chunks
}

func chunkRuneEnds(text string) []int {
	ends := make([]int, 0, utf8.RuneCountInString(text))
	for offset := 0; offset < len(text); {
		_, size := utf8.DecodeRuneInString(text[offset:])
		offset += size
		ends = append(ends, offset)
	}
	return ends
}

func chunksWithinLimit(chunks []string, limit int, lenFn func(string) int) bool {
	for _, chunk := range chunks {
		if lenFn(chunk) > limit {
			return false
		}
	}
	return true
}

func appendChunkIndicators(chunks []string) []string {
	if len(chunks) <= 1 {
		return chunks
	}
	result := make([]string, len(chunks))
	for index, chunk := range chunks {
		result[index] = chunk + chunkIndicator(index+1, len(chunks))
	}
	return result
}

func chunkIndicator(index int, total int) string {
	return fmt.Sprintf("\n\n(%d/%d)", index, total)
}

func messageRuneLen(value string) int {
	return utf8.RuneCountInString(value)
}
