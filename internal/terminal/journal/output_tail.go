package journal

import terminalpkg "github.com/compozy/compozy/internal/terminal"

func (l *terminalLane) appendOutputTailLocked(output []byte) {
	if len(output) == 0 {
		return
	}
	l.appendOutputSegmentLocked(terminalpkg.OutputSegment{Kind: terminalpkg.OutputSegmentBytes, Text: string(output)})
}

func (l *terminalLane) appendOutputSegmentLocked(segment terminalpkg.OutputSegment) {
	if segment.Kind == terminalpkg.OutputSegmentBytes && segment.Text == "" {
		return
	}
	if segment.Kind == terminalpkg.OutputSegmentBytes && len(l.outputTail) > 0 &&
		l.outputTail[len(l.outputTail)-1].Kind == terminalpkg.OutputSegmentBytes {
		l.outputTail[len(l.outputTail)-1].Text += segment.Text
	} else {
		l.outputTail = append(l.outputTail, segment)
	}
	l.outputTail = terminalpkg.BoundedOutputTail(l.outputTail)
}

func (l *terminalLane) takeOutputTail() []terminalpkg.OutputSegment {
	l.mu.Lock()
	defer l.mu.Unlock()
	segments := append([]terminalpkg.OutputSegment(nil), l.outputTail...)
	l.outputTail = nil
	return segments
}
