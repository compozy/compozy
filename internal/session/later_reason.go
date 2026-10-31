package session

import (
	"regexp"
	"strings"
)

// laterReasonLineSeparator joins the lines of a multi-line reason when it is
// read as one line (a notification body).
const laterReasonLineSeparator = " · "

var (
	laterReasonBlockMarker = regexp.MustCompile(`^(?:#{1,6}\s+|>\s?|[-*+]\s+(?:\[[ xX]\]\s+)?|\d+[.)]\s+)`)
	laterReasonLink        = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	laterReasonCode        = regexp.MustCompile("`([^`]*)`")
	laterReasonStrong      = regexp.MustCompile(`\*\*(\S(?:.*?\S)?)\*\*`)
	laterReasonStrongAlt   = regexp.MustCompile(`__(\S(?:.*?\S)?)__`)
	laterReasonStrike      = regexp.MustCompile(`~~(\S(?:.*?\S)?)~~`)
	laterReasonEmphasis    = regexp.MustCompile(`\*(\S(?:[^*]*?\S)?)\*`)
	laterReasonEscape      = regexp.MustCompile("\\\\([\\\\`*_{}\\[\\]()#+\\-.!~|>])")
)

// LaterReasonPlainText reads a Markdown Later reason as one plain line for
// surfaces that cannot render Markdown: block markers and inline delimiters
// drop, link text stays, and lines join with a middle dot. Plain-text reasons
// written before reasons became Markdown read unchanged.
func LaterReasonPlainText(reason string) string {
	lines := strings.Split(strings.ReplaceAll(reason, "\r\n", "\n"), "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		text := laterReasonBlockMarker.ReplaceAllString(strings.TrimSpace(line), "")
		text = laterReasonLink.ReplaceAllString(text, "$1")
		text = laterReasonCode.ReplaceAllString(text, "$1")
		text = laterReasonStrong.ReplaceAllString(text, "$1")
		text = laterReasonStrongAlt.ReplaceAllString(text, "$1")
		text = laterReasonStrike.ReplaceAllString(text, "$1")
		text = laterReasonEmphasis.ReplaceAllString(text, "$1")
		text = strings.TrimSpace(laterReasonEscape.ReplaceAllString(text, "$1"))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, laterReasonLineSeparator)
}
