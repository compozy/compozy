package run

import (
	"io"
	"sync"
	"time"
)

type lineRing struct {
	mu    sync.Mutex
	capN  int
	lines []string
}

func newLineRing(n int) *lineRing {
	if n <= 0 {
		n = 1
	}
	return &lineRing{capN: n, lines: make([]string, 0, n)}
}

func (r *lineRing) appendLine(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s == "" {
		return
	}
	r.lines = append(r.lines, s)
	if len(r.lines) > r.capN {
		r.lines = r.lines[len(r.lines)-r.capN:]
	}
}

func (r *lineRing) snapshot() []string {
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
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastActivity = time.Now()
}

func (a *activityMonitor) timeSinceLastActivity() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Since(a.lastActivity)
}

type activityWriter struct {
	dst     io.Writer
	monitor *activityMonitor
}

func newActivityWriter(dst io.Writer, monitor *activityMonitor) io.Writer {
	if dst == nil {
		return io.Discard
	}
	return &activityWriter{dst: dst, monitor: monitor}
}

func (w *activityWriter) Write(p []byte) (int, error) {
	if len(p) > 0 && w.monitor != nil {
		w.monitor.recordActivity()
	}
	return w.dst.Write(p)
}
