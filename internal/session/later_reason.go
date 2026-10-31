package session

import (
	"regexp"
	"strconv"
	"strings"
)

// laterReasonLineSeparator joins the lines of a multi-line reason when it is
// read as one line (a notification body).
const laterReasonLineSeparator = " · "

// Literal runs (code spans, backslash escapes) are parked behind private-use
// placeholders while the formatting passes run, then restored verbatim.
const (
	laterReasonSlotOpen  = ""
	laterReasonSlotClose = ""
)

var (
	laterReasonBlockMarker = regexp.MustCompile(`^(?:#{1,6}\s+|>\s?|[-*+]\s+(?:\[[ xX]\]\s+)?|\d+[.)]\s+)`)
	laterReasonCode        = regexp.MustCompile("`([^`]*)`")
	laterReasonEscape      = regexp.MustCompile("\\\\([\\\\`*_{}\\[\\]()#+\\-.!~|>])")
	laterReasonLink        = regexp.MustCompile(`!?\[([^\]]*)\]\((?:[^()]|\([^()]*\))*\)`)
	laterReasonStrong      = regexp.MustCompile(`\*\*(\S(?:.*?\S)?)\*\*`)
	laterReasonStrongAlt   = regexp.MustCompile(`__(\S(?:.*?\S)?)__`)
	laterReasonStrike      = regexp.MustCompile(`~~(\S(?:.*?\S)?)~~`)
	laterReasonEmphasis    = regexp.MustCompile(`\*(\S(?:[^*]*?\S)?)\*`)
	laterReasonSlot        = regexp.MustCompile(laterReasonSlotOpen + `(\d+)` + laterReasonSlotClose)
)

// LaterReasonPlainText reads a Markdown Later reason as one plain line for
// surfaces that cannot render Markdown: block markers and inline delimiters
// drop, link text stays, code spans and escaped characters stay literal, and
// lines join with a middle dot. The stored reason is never rewritten.
func LaterReasonPlainText(reason string) string {
	lines := strings.Split(strings.ReplaceAll(reason, "\r\n", "\n"), "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		if text := laterReasonLinePlainText(line); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, laterReasonLineSeparator)
}

func laterReasonLinePlainText(line string) string {
	var literals []string
	park := func(literal string) string {
		literals = append(literals, literal)
		return laterReasonSlotOpen + strconv.Itoa(len(literals)-1) + laterReasonSlotClose
	}
	text := laterReasonBlockMarker.ReplaceAllString(strings.TrimSpace(line), "")
	text = laterReasonCode.ReplaceAllStringFunc(text, func(span string) string {
		return park(laterReasonCode.FindStringSubmatch(span)[1])
	})
	text = laterReasonEscape.ReplaceAllStringFunc(text, func(escaped string) string {
		return park(escaped[1:])
	})
	text = laterReasonLink.ReplaceAllString(text, "$1")
	text = laterReasonStrong.ReplaceAllString(text, "$1")
	text = laterReasonStrongAlt.ReplaceAllString(text, "$1")
	text = laterReasonStrike.ReplaceAllString(text, "$1")
	text = laterReasonEmphasis.ReplaceAllString(text, "$1")
	text = laterReasonSlot.ReplaceAllStringFunc(text, func(slot string) string {
		index, err := strconv.Atoi(laterReasonSlot.FindStringSubmatch(slot)[1])
		if err != nil || index >= len(literals) {
			return slot
		}
		return literals[index]
	})
	return strings.TrimSpace(text)
}
