package acp

import "sync"

type lockedBuffer struct {
	mu sync.Mutex
	b  []byte
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.b = appendBounded(b.b, p, defaultTerminalOutputLimit)
	return len(p), nil
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.b)
}

func appendBounded(dst []byte, src []byte, limit int) []byte {
	if limit <= 0 {
		return nil
	}

	dst = append(dst[:len(dst):len(dst)], src...)
	combined := dst
	if len(combined) <= limit {
		return combined
	}

	return append([]byte(nil), combined[len(combined)-limit:]...)
}
