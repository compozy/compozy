package terminal

import (
	"bytes"
	"regexp"
	"regexp/syntax"
	"slices"
)

// matchScanner evaluates a match wait against the retained output without
// re-rendering the whole ring on every revision when that is provably safe.
//
// A pattern that cannot match a newline and has no begin-of-text anchor only
// ever matches inside one line, so after a scan only the last (possibly
// incomplete) line and the output appended since can produce a new match. The
// scanner rescans from that line start: exact, and O(new output) per revision.
// Every other pattern scans the full retained output each revision.
type matchScanner struct {
	matcher   *regexp.Regexp
	lineLocal bool
	scanned   bool
	next      uint64
	lineStart uint64
}

func newMatchScanner(matcher *regexp.Regexp) *matchScanner {
	if matcher == nil {
		return &matchScanner{}
	}
	return &matchScanner{matcher: matcher, lineLocal: lineLocalPattern(matcher.String())}
}

func (m *matchScanner) matches(ring *Ring) bool {
	if m.matcher == nil {
		return false
	}
	if !m.lineLocal {
		output, _ := ring.Snapshot()
		return m.matcher.Match(output)
	}
	oldest, next := ring.Bounds()
	if m.scanned && next == m.next {
		return false
	}
	from := max(m.lineStart, oldest)
	replay := ring.ReplayFrom(from)
	m.scanned, m.next = true, replay.Seq
	m.lineStart = replayLastLineStart(replay, from)
	return m.matcher.Match(replay.Bytes())
}

// replayLastLineStart returns the sequence just after the last newline in the
// replayed byte output, or from when the replay holds no newline.
func replayLastLineStart(replay Replay, from uint64) uint64 {
	for _, segment := range slices.Backward(replay.Segments) {
		if segment.Segment.Kind != OutputSegmentBytes {
			continue
		}
		if newline := bytes.LastIndexByte([]byte(segment.Segment.Text), '\n'); newline >= 0 {
			// #nosec G115 -- newline is a non-negative index into the segment text.
			return segment.Seq + uint64(newline) + 1
		}
	}
	return from
}

// lineLocalPattern reports whether every match of pattern lies inside a
// single line and does not depend on where the scanned text begins.
func lineLocalPattern(pattern string) bool {
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return false
	}
	return lineLocalRegexp(parsed)
}

func lineLocalRegexp(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpAnyChar, syntax.OpBeginText:
		return false
	case syntax.OpLiteral:
		if slices.Contains(re.Rune, '\n') {
			return false
		}
	case syntax.OpCharClass:
		for pair := range slices.Chunk(re.Rune, 2) {
			if len(pair) == 2 && pair[0] <= '\n' && '\n' <= pair[1] {
				return false
			}
		}
	}
	for _, sub := range re.Sub {
		if !lineLocalRegexp(sub) {
			return false
		}
	}
	return true
}
