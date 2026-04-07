package run

import (
	"io"
	"os"
	"sync"
	"time"
)

type lineBuffer struct {
	mu    sync.Mutex
	capN  int
	lines []string
}

func newLineBuffer(n int) *lineBuffer {
	if n < 0 {
		n = 0
	}
	initialCap := n
	if initialCap <= 0 {
		initialCap = 32
	}
	return &lineBuffer{capN: n, lines: make([]string, 0, initialCap)}
}

func (r *lineBuffer) appendLine(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, s)
	if r.capN > 0 && len(r.lines) > r.capN {
		r.lines = r.lines[len(r.lines)-r.capN:]
	}
}

func (r *lineBuffer) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

type activityMonitor struct {
	mu           sync.Mutex
	lastActivity time.Time
}

func newActivityMonitor() *activityMonitor {
	return &activityMonitor{lastActivity: time.Now()}
}

func (a *activityMonitor) recordActivity() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastActivity = time.Now()
}

func (a *activityMonitor) timeSinceLastActivity() time.Duration {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Since(a.lastActivity)
}

func appendLinesToBuffer(buf *lineBuffer, lines []string) {
	if buf == nil {
		return
	}
	for _, line := range lines {
		buf.appendLine(line)
	}
}

func createLogWriters(outFile *os.File, errFile *os.File, useUI bool, emitHuman bool) (io.Writer, io.Writer) {
	if useUI || !emitHuman {
		return writerOrNil(outFile), writerOrNil(errFile)
	}
	return io.MultiWriter(writerOrNil(outFile), os.Stdout), io.MultiWriter(writerOrNil(errFile), os.Stderr)
}

func writerOrNil(file *os.File) io.Writer {
	if file == nil {
		return nil
	}
	return file
}
