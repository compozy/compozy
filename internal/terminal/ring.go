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
	segments []retainedOutputSegment
	bytes    int
	capacity int
	oldest   uint64
	next     uint64
	preamble []byte
	modes    *modePreambleTracker
}

type retainedOutputSegment struct {
	seq, end   uint64
	kind       OutputSegmentKind
	data       []byte
	characters int
}

func (s retainedOutputSegment) output() OutputSegment {
	if s.kind == OutputSegmentBytes {
		return OutputSegment{Kind: s.kind, Text: string(s.data)}
	}
	return RedactedInputMarker(s.characters)
}

func (s retainedOutputSegment) cost() int {
	if s.kind == OutputSegmentBytes {
		return len(s.data)
	}
	return len(RenderOutputSegment(s.output()))
}

func NewRing(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{capacity: capacity, segments: make([]retainedOutputSegment, 0, 8), modes: newModePreambleTracker()}
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
	r.appendSegmentLocked(retainedOutputSegment{
		seq: start, end: r.next, kind: OutputSegmentBytes, data: input,
	})
	r.trim()
	return start, r.next
}

func (r *Ring) AppendRedactedInput(characters int) (start uint64, end uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	start = r.next
	r.next++
	r.appendSegmentLocked(retainedOutputSegment{
		seq: start, end: r.next, kind: OutputSegmentRedactedInput, characters: max(characters, 0),
	})
	r.trim()
	return start, r.next
}

func (r *Ring) appendSegmentLocked(segment retainedOutputSegment) {
	cost := segment.cost()
	if segment.kind == OutputSegmentBytes && len(r.segments) > 0 {
		last := &r.segments[len(r.segments)-1]
		if last.kind == OutputSegmentBytes && last.end == segment.seq {
			last.end = segment.end
			last.data = append(last.data, segment.data...)
			r.bytes += cost
			return
		}
	}
	segment.data = bytes.Clone(segment.data)
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
	var preamble []byte
	truncated := seq < r.oldest
	if truncated {
		preamble = make([]byte, 0, len(resetSequence)+len(r.preamble))
		preamble = append(preamble, resetSequence...)
		preamble = append(preamble, r.preamble...)
	}
	seq = min(seq, r.next)
	segments := make([]SequencedOutputSegment, 0, len(r.segments))
	for _, segment := range r.segments {
		if segment.end <= seq {
			continue
		}
		if segment.kind == OutputSegmentBytes && seq > segment.seq {
			offset := min(seq-segment.seq, uint64(len(segment.data)))
			// #nosec G115 -- offset is bounded by the segment data length.
			segment.data = segment.data[int(offset):]
			segment.seq = seq
		}
		segments = append(segments, SequencedOutputSegment{
			Seq: segment.seq, End: segment.end, Segment: segment.output(),
		})
	}
	return Replay{Preamble: preamble, Segments: segments, Seq: r.next, Truncated: truncated}
}

func (r *Ring) Snapshot() ([]byte, uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	content := make([]byte, 0, r.bytes)
	for _, segment := range r.segments {
		if segment.kind == OutputSegmentBytes {
			content = append(content, segment.data...)
		} else {
			content = append(content, RenderOutputSegment(segment.output())...)
		}
	}
	return content, r.next
}

func (r *Ring) SnapshotSegments() ([]OutputSegment, uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	segments := make([]OutputSegment, 0, len(r.segments))
	for _, segment := range r.segments {
		segments = append(segments, segment.output())
	}
	return segments, r.next
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
		cost := first.cost()
		if cost <= excess || first.kind != OutputSegmentBytes {
			r.bytes -= cost
			r.oldest = first.end
			*first = retainedOutputSegment{}
			r.segments = r.segments[1:]
			continue
		}
		data := first.data
		cut := safeTrimIndex(data, excess)
		if cut < 1 {
			cut = excess
		}
		for cut < len(data) && data[cut]&0xc0 == 0x80 {
			cut++
		}
		first.data = data[cut:]
		if len(data)-r.capacity > r.capacity {
			first.data = bytes.Clone(first.data)
		}
		// #nosec G115 -- cut is bounded by the segment text length.
		first.seq += uint64(cut)
		r.oldest = first.seq
		r.bytes -= cut
	}
}

func safeTrimIndex(data []byte, minimum int) int {
	if minimum >= len(data) {
		return len(data)
	}
	state := byte(0)
	for index, value := range data {
		state = nextEscapeState(state, value)
		if index+1 >= minimum && state == 0 {
			if newline := bytes.IndexByte(data[index+1:min(index+257, len(data))], '\n'); newline >= 0 {
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
