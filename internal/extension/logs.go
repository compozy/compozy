package extensionpkg

import (
	"sync"
	"time"
)

const extensionLogRingCapacity = 256 * 1024

// ExtensionLogEntry is one redacted stderr record retained for an instance.
type ExtensionLogEntry struct {
	Sequence       int64     `json:"sequence"`
	Timestamp      time.Time `json:"timestamp"`
	Message        string    `json:"message"`
	GenerationHash string    `json:"generation_hash,omitempty"`
}

// ExtensionLogRing is a bounded, concurrency-safe, drop-oldest byte ring.
type ExtensionLogRing struct {
	mu       sync.RWMutex
	capacity int
	bytes    int
	next     int64
	now      func() time.Time
	entries  []ExtensionLogEntry
}

func newExtensionLogRing(now func() time.Time) *ExtensionLogRing {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &ExtensionLogRing{capacity: extensionLogRingCapacity, now: now}
}

func (r *ExtensionLogRing) append(message, generationHash string) {
	if r == nil || r.capacity <= 0 || message == "" {
		return
	}
	if len(message) > r.capacity {
		message = message[len(message)-r.capacity:]
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	entry := ExtensionLogEntry{
		Sequence:       r.next,
		Timestamp:      r.now().UTC(),
		Message:        message,
		GenerationHash: generationHash,
	}
	r.entries = append(r.entries, entry)
	r.bytes += len(message)
	for r.bytes > r.capacity && len(r.entries) > 1 {
		r.bytes -= len(r.entries[0].Message)
		copy(r.entries, r.entries[1:])
		r.entries = r.entries[:len(r.entries)-1]
	}
}

func (r *ExtensionLogRing) snapshot(after int64) []ExtensionLogEntry {
	if r == nil {
		return []ExtensionLogEntry{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := make([]ExtensionLogEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		if entry.Sequence <= after {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}
