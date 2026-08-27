package terminal

import (
	"bytes"
	"sync"
)

const resetSequence = "\x1bc"

type Replay struct {
	Payload   []byte
	Seq       uint64
	Truncated bool
}

type Ring struct {
	mu       sync.RWMutex
	data     []byte
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
	return &Ring{capacity: capacity, data: make([]byte, 0, capacity), modes: newModePreambleTracker()}
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
	r.data = append(r.data, input...)
	r.next += uint64(len(input))
	r.trim()
	return start, r.next
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
		payload := make([]byte, 0, len(resetSequence)+len(r.preamble)+len(r.data))
		payload = append(payload, resetSequence...)
		payload = append(payload, r.preamble...)
		payload = append(payload, r.data...)
		return Replay{Payload: payload, Seq: r.next, Truncated: true}
	}
	if seq > r.next {
		seq = r.next
	}
	delta := min(seq-r.oldest, uint64(len(r.data)))
	// #nosec G115 -- delta is bounded by len(r.data), which is representable as int.
	offset := int(delta)
	return Replay{Payload: append([]byte(nil), r.data[offset:]...), Seq: r.next}
}

func (r *Ring) Snapshot() ([]byte, uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]byte(nil), r.data...), r.next
}

func (r *Ring) Bounds() (oldest uint64, next uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.oldest, r.next
}

func (r *Ring) trim() {
	if len(r.data) <= r.capacity {
		return
	}
	excess := len(r.data) - r.capacity
	cut := safeTrimIndex(r.data, excess)
	if cut < 1 {
		cut = excess
	}
	for cut < len(r.data) && r.data[cut]&0xc0 == 0x80 {
		cut++
	}
	// #nosec G115 -- cut is a nonnegative index bounded by len(r.data).
	r.oldest += uint64(cut)
	remaining := len(r.data) - cut
	copy(r.data[:remaining], r.data[cut:])
	r.data = r.data[:remaining]
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
