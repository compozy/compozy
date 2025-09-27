package exec

import "bytes"

type limitedBuffer struct {
	limit     int64
	buffer    bytes.Buffer
	truncated bool
	written   int64
}

func newLimitedBuffer(limit int64) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.written += int64(len(p))
	if b.limit <= 0 {
		return b.buffer.Write(p)
	}
	remaining := b.limit - int64(b.buffer.Len())
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if int64(len(p)) > remaining {
		limit := int(remaining)
		_, _ = b.buffer.Write(p[:limit])
		b.truncated = true
		return len(p), nil
	}
	return b.buffer.Write(p)
}

func (b *limitedBuffer) String() string {
	return b.buffer.String()
}

func (b *limitedBuffer) Truncated() bool {
	return b.truncated
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buffer.Bytes()
}

func (b *limitedBuffer) Written() int64 {
	return b.written
}
