package terminal

import (
	"bytes"
	"sync"
)

const resetSequence = "\x1bc"

type Replay struct {
	Preamble  []byte
	Segments  []SequencedOutputSegment
	Seq       uint64
	Truncated bool
}

type SequencedOutputSegment struct {
	Seq     uint64
	End     uint64
	Segment OutputSegment
}

func (r Replay) Bytes() []byte {
	content := append([]byte(nil), r.Preamble...)
	for _, segment := range r.Segments {
		content = append(content, RenderOutputSegment(segment.Segment)...)
	}
	return content
}

type Ring struct {
	mu       sync.RWMutex
	segments []SequencedOutputSegment
	bytes    int
	capacity int
	oldest   uint64
	next     uint64
	preamble []byte
	modes    *modePreambleTracker
}

func NewRing(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{capacity: capacity, segments: make([]SequencedOutputSegment, 0, 8), modes: newModePreambleTracker()}
}

func (r *Ring) Append(input []byte) (start uint64, end uint64) {
	if len(input) == 0 {
		r.mu.RLock()
		defer r.mu.RUnlock()
		return r.next, r.next
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	start = r.next
	r.next += uint64(len(input))
	r.appendSegmentLocked(SequencedOutputSegment{
		Seq: start, End: r.next,
		Segment: OutputSegment{Kind: OutputSegmentBytes, Text: string(input)},
	})
	r.trim()
	return start, r.next
}

func (r *Ring) AppendRedactedInput(characters int) (start uint64, end uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	start = r.next
	r.next++
	r.appendSegmentLocked(SequencedOutputSegment{
		Seq: start, End: r.next, Segment: RedactedInputMarker(characters),
	})
	r.trim()
	return start, r.next
}

func (r *Ring) appendSegmentLocked(segment SequencedOutputSegment) {
	cost := len(RenderOutputSegment(segment.Segment))
	if segment.Segment.Kind == OutputSegmentBytes && len(r.segments) > 0 {
		last := &r.segments[len(r.segments)-1]
		if last.Segment.Kind == OutputSegmentBytes && last.End == segment.Seq {
			last.End = segment.End
			last.Segment.Text += segment.Segment.Text
			r.bytes += cost
			return
		}
	}
	r.segments = append(r.segments, segment)
	r.bytes += cost
}

func (r *Ring) SetModePreamble(preamble []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.preamble = append(r.preamble[:0], r.modes.Observe(preamble)...)
}

func (r *Ring) ReplayFrom(seq uint64) Replay {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if seq < r.oldest {
		preamble := make([]byte, 0, len(resetSequence)+len(r.preamble))
		preamble = append(preamble, resetSequence...)
		preamble = append(preamble, r.preamble...)
		return Replay{
			Preamble: preamble, Segments: cloneSequencedOutputSegments(r.segments),
			Seq: r.next, Truncated: true,
		}
	}
	if seq > r.next {
		seq = r.next
	}
	segments := make([]SequencedOutputSegment, 0, len(r.segments))
	for _, segment := range r.segments {
		if segment.End <= seq {
			continue
		}
		copyOfSegment := segment
		if segment.Segment.Kind == OutputSegmentBytes && seq > segment.Seq {
			offset := min(seq-segment.Seq, uint64(len(segment.Segment.Text)))
			// #nosec G115 -- offset is bounded by the segment text length.
			copyOfSegment.Segment.Text = segment.Segment.Text[int(offset):]
			copyOfSegment.Seq = seq
		}
		segments = append(segments, copyOfSegment)
	}
	return Replay{Segments: segments, Seq: r.next}
}

func (r *Ring) Snapshot() ([]byte, uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return renderSequencedOutputSegments(r.segments), r.next
}

func (r *Ring) SnapshotSegments() ([]OutputSegment, uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	segments := make([]OutputSegment, 0, len(r.segments))
	for _, segment := range r.segments {
		segments = append(segments, segment.Segment)
	}
	return cloneOutputSegments(segments), r.next
}

func (r *Ring) Bounds() (oldest uint64, next uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.oldest, r.next
}

func (r *Ring) trim() {
	for r.bytes > r.capacity && len(r.segments) > 0 {
		excess := r.bytes - r.capacity
		first := &r.segments[0]
		cost := len(RenderOutputSegment(first.Segment))
		if cost <= excess || first.Segment.Kind != OutputSegmentBytes {
			r.bytes -= cost
			r.oldest = first.End
			r.segments = r.segments[1:]
			continue
		}
		data := []byte(first.Segment.Text)
		cut := safeTrimIndex(data, excess)
		if cut < 1 {
			cut = excess
		}
		for cut < len(data) && data[cut]&0xc0 == 0x80 {
			cut++
		}
		first.Segment.Text = string(data[cut:])
		// #nosec G115 -- cut is bounded by the segment text length.
		first.Seq += uint64(cut)
		r.oldest = first.Seq
		r.bytes -= cut
	}
}

func cloneSequencedOutputSegments(segments []SequencedOutputSegment) []SequencedOutputSegment {
	return append([]SequencedOutputSegment(nil), segments...)
}

func renderSequencedOutputSegments(segments []SequencedOutputSegment) []byte {
	var content []byte
	for _, segment := range segments {
		content = append(content, RenderOutputSegment(segment.Segment)...)
	}
	return content
}

func safeTrimIndex(data []byte, minimum int) int {
	if minimum >= len(data) {
		return len(data)
	}
	state := byte(0)
	for index, value := range data {
		state = nextEscapeState(state, value)
		if index+1 >= minimum && state == 0 {
			if newline := bytes.IndexByte(data[index+1:], '\n'); newline >= 0 && newline < 256 {
				return index + newline + 2
			}
			return index + 1
		}
	}
	// The retained suffix would otherwise begin inside an incomplete escape.
	// Dropping the incomplete suffix preserves both the cap and replay safety.
	return len(data)
}

func nextEscapeState(state, value byte) byte {
	switch state {
	case 0:
		if value == 0x1b {
			return 1
		}
	case 1:
		switch value {
		case ']':
			return 2
		case 'P':
			return 3
		case '[':
			return 4
		}
		return 0
	case 2, 3:
		if value == 0x07 {
			return 0
		}
		if value == 0x1b {
			return state + 4
		}
	case 4:
		if value >= 0x40 && value <= 0x7e {
			return 0
		}
	case 6, 7:
		if value == '\\' {
			return 0
		}
		if value != 0x1b {
			return state - 4
		}
	}
	return state
}
