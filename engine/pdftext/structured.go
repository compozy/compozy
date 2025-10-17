package pdftext

import (
	"context"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
)

func (e *Extractor) extractStructured(
	ctx context.Context,
	instance pdfium.Pdfium,
	doc *responses.OpenDocument,
	runeLimit int64,
	pageCount int,
) (string, Stats, error) {
	limiter := newTextLimiter(runeLimit)
	allOutputs := make([]*lineOutput, 0, pageCount*16)

	for page := range pageCount {
		if err := ctx.Err(); err != nil {
			return "", Stats{}, err
		}
		resp, err := instance.GetPageTextStructured(&requests.GetPageTextStructured{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc.Document,
					Index:    page,
				},
			},
			Mode: requests.GetPageTextStructuredModeRects,
		})
		if err != nil {
			return "", Stats{}, err
		}
		segments := collectSegments(resp.Rects)
		if len(segments) == 0 {
			continue
		}
		groups := groupSegments(segments)
		pageOutputs := make([]*lineOutput, 0, len(groups))
		for _, group := range groups {
			if output := group.toOutput(); output != nil {
				pageOutputs = append(pageOutputs, output)
			}
		}
		allOutputs = append(allOutputs, pageOutputs...)
		if page < pageCount-1 {
			allOutputs = append(allOutputs, &lineOutput{separatorHint: "\n\n"})
		}
	}

	finalLines := finalizeLineOutputs(allOutputs)
	for _, line := range finalLines {
		if line.separator != "" && limiter.AppendString(line.separator) {
			break
		}
		if line.text != "" && limiter.AppendString(line.text) {
			break
		}
	}

	text := strings.TrimSpace(limiter.String())
	return text, computeStats(text, true), nil
}

func collectSegments(rects []*responses.GetPageTextStructuredRect) []*rectSegment {
	segments := make([]*rectSegment, 0, len(rects))
	for _, rect := range rects {
		if rect == nil {
			continue
		}
		text := normalizeRectText(rect.Text)
		if text == "" {
			continue
		}
		charCount := utf8.RuneCountInString(text)
		if charCount == 0 {
			continue
		}
		seg := &rectSegment{
			text:      text,
			left:      rect.PointPosition.Left,
			right:     rect.PointPosition.Right,
			top:       rect.PointPosition.Top,
			bottom:    rect.PointPosition.Bottom,
			charCount: charCount,
		}
		segments = append(segments, seg)
	}
	sort.Slice(segments, func(i, j int) bool {
		if almostEqual(segments[i].top, segments[j].top) {
			return segments[i].left < segments[j].left
		}
		return segments[i].top < segments[j].top
	})
	return segments
}

func groupSegments(segments []*rectSegment) []*lineGroup {
	lines := make([]*lineGroup, 0, len(segments)/2+1)
	for _, seg := range segments {
		assigned := false
		for _, line := range lines {
			if line.accepts(seg) {
				line.add(seg)
				assigned = true
				break
			}
		}
		if !assigned {
			lines = append(lines, newLineGroup(seg))
		}
	}
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].top < lines[j].top
	})
	return lines
}

func finalizeLineOutputs(outputs []*lineOutput) []finalLine {
	final := make([]finalLine, 0, len(outputs))
	pendingSeparator := ""

	for _, out := range outputs {
		if out == nil {
			continue
		}
		text := strings.TrimSpace(out.text)
		if out.separatorHint != "" && text == "" {
			pendingSeparator = out.separatorHint
			continue
		}
		if text == "" {
			continue
		}
		if len(final) == 0 {
			final = append(final, finalLine{text: text, top: out.top, bottom: out.bottom, height: out.height})
			continue
		}
		prev := &final[len(final)-1]
		if shouldMergeHyphen(prev.text, text) {
			prev.text = strings.TrimSuffix(strings.TrimRight(prev.text, " "), "-")
			prev.text += strings.TrimLeft(text, " ")
			if out.bottom > prev.bottom {
				prev.bottom = out.bottom
			}
			if out.height > prev.height {
				prev.height = out.height
			}
			continue
		}
		sep := pendingSeparator
		pendingSeparator = ""
		if sep == "" {
			gap := out.top - prev.bottom
			if gap > paragraphGapThreshold(prev.height, out.height) {
				sep = "\n\n"
			} else {
				sep = "\n"
			}
		}
		final = append(final, finalLine{
			separator: sep,
			text:      text,
			top:       out.top,
			bottom:    out.bottom,
			height:    out.height,
		})
	}
	return final
}

type rectSegment struct {
	text      string
	left      float64
	right     float64
	top       float64
	bottom    float64
	charCount int
}

func (r *rectSegment) width() float64 {
	return r.right - r.left
}

func (r *rectSegment) charWidth() float64 {
	width := r.width()
	if width <= 0 || r.charCount <= 0 {
		return 0
	}
	return width / float64(r.charCount)
}

type lineGroup struct {
	segments []*rectSegment
	top      float64
	bottom   float64
	height   float64
}

func newLineGroup(seg *rectSegment) *lineGroup {
	height := seg.bottom - seg.top
	if height <= 0 {
		height = 1
	}
	return &lineGroup{
		segments: []*rectSegment{seg},
		top:      seg.top,
		bottom:   seg.bottom,
		height:   height,
	}
}

func (l *lineGroup) accepts(seg *rectSegment) bool {
	tolerance := math.Max(l.height*0.35, 1.0)
	if math.Abs(seg.top-l.top) <= tolerance || math.Abs(seg.bottom-l.top) <= tolerance {
		return true
	}
	midCurrent := (l.top + l.bottom) / 2
	midCandidate := (seg.top + seg.bottom) / 2
	return math.Abs(midCandidate-midCurrent) <= tolerance
}

func (l *lineGroup) add(seg *rectSegment) {
	l.segments = append(l.segments, seg)
	if seg.top < l.top {
		l.top = seg.top
	}
	if seg.bottom > l.bottom {
		l.bottom = seg.bottom
	}
	height := seg.bottom - seg.top
	if height <= 0 {
		height = 1
	}
	l.height = (l.height*float64(len(l.segments)-1) + height) / float64(len(l.segments))
}

func (l *lineGroup) toOutput() *lineOutput {
	if len(l.segments) == 0 {
		return nil
	}
	sort.Slice(l.segments, func(i, j int) bool {
		if almostEqual(l.segments[i].left, l.segments[j].left) {
			return l.segments[i].width() > l.segments[j].width()
		}
		return l.segments[i].left < l.segments[j].left
	})
	var builder strings.Builder
	var prev *rectSegment
	var lastRune rune
	var hasLast bool
	for _, seg := range l.segments {
		text := seg.text
		if text == "" {
			continue
		}
		if prev != nil && needsSpace(prev, seg, lastRune, hasLast) {
			builder.WriteByte(' ')
			lastRune = ' '
			hasLast = true
		}
		builder.WriteString(text)
		if r, size := utf8.DecodeLastRuneInString(text); size > 0 {
			lastRune = r
			hasLast = true
		}
		prev = seg
	}
	lineText := strings.TrimSpace(builder.String())
	if lineText == "" {
		return nil
	}
	height := l.height
	if height <= 0 {
		height = 1
	}
	return &lineOutput{
		text:   lineText,
		top:    l.top,
		bottom: l.bottom,
		height: height,
	}
}

func needsSpace(prev, curr *rectSegment, lastRune rune, hasLast bool) bool {
	if strings.HasPrefix(curr.text, " ") || strings.HasSuffix(prev.text, " ") {
		return false
	}
	if hasLast && unicode.IsSpace(lastRune) {
		return false
	}
	gap := curr.left - prev.right
	if gap <= 0 {
		return false
	}
	width := math.Max(prev.charWidth(), curr.charWidth())
	if width <= 0 {
		width = 1
	}
	return gap > width*0.45
}

type lineOutput struct {
	text          string
	top           float64
	bottom        float64
	height        float64
	separatorHint string
}

type finalLine struct {
	separator string
	text      string
	top       float64
	bottom    float64
	height    float64
}

func shouldMergeHyphen(prevLine, nextLine string) bool {
	prevTrim := strings.TrimRight(prevLine, " ")
	if !strings.HasSuffix(prevTrim, "-") {
		return false
	}
	nextTrim := strings.TrimLeft(nextLine, " ")
	if nextTrim == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(nextTrim)
	return unicode.IsLetter(r) && unicode.IsLower(r)
}

func paragraphGapThreshold(prevHeight, currHeight float64) float64 {
	base := math.Max(prevHeight, currHeight)
	if base <= 0 {
		base = 1
	}
	return base * 1.4
}

func normalizeRectText(s string) string {
	if s == "" {
		return s
	}
	replacer := strings.NewReplacer(
		"\r\n", " ",
		"\r", " ",
		"\n", " ",
		"\t", " ",
		"\u00a0", " ",
	)
	return strings.TrimSpace(replacer.Replace(s))
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= 0.5
}
