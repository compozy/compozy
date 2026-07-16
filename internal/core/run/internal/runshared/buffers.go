package runshared

import (
	"io"
	"os"
	"sync"
	"time"
)

type LineBuffer struct {
	mu    sync.Mutex
	capN  int
	lines []string
}

func NewLineBuffer(n int) *LineBuffer {
	if n < 0 {
		n = 0
	}
	initialCap := n
	if initialCap <= 0 {
		initialCap = 32
	}
	return &LineBuffer{capN: n, lines: make([]string, 0, initialCap)}
}

func (r *LineBuffer) AppendLine(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, s)
	if r.capN > 0 && len(r.lines) > r.capN {
		r.lines = r.lines[len(r.lines)-r.capN:]
	}
}

func (r *LineBuffer) Snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

type ActivityMonitor struct {
	mu           sync.Mutex
	lastActivity time.Time
	active       int
	clock        Clock
}

func NewActivityMonitor() *ActivityMonitor {
	return NewActivityMonitorWithClock(RealClock{})
}

// NewActivityMonitorWithClock builds a monitor whose idle window advances on the
// injected clock, so the stall watchdog can be tested without real sleeps.
func NewActivityMonitorWithClock(clock Clock) *ActivityMonitor {
	if clock == nil {
		clock = RealClock{}
	}
	return &ActivityMonitor{lastActivity: clock.Now(), clock: clock}
}

// now returns the monitor's clock time, defaulting to wall time when the monitor
// was constructed without an explicit clock (e.g. a zero-value literal).
func (a *ActivityMonitor) now() time.Time {
	if a.clock != nil {
		return a.clock.Now()
	}
	return time.Now()
}

func (a *ActivityMonitor) RecordActivity() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastActivity = a.now()
}

func (a *ActivityMonitor) BeginActivity() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.active++
	a.lastActivity = a.now()
}

func (a *ActivityMonitor) EndActivity() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active > 0 {
		a.active--
	}
	a.lastActivity = a.now()
}

func (a *ActivityMonitor) TimeSinceLastActivity() time.Duration {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active > 0 {
		return 0
	}
	return a.now().Sub(a.lastActivity)
}

func AppendLinesToBuffer(buf *LineBuffer, lines []string) {
	if buf == nil {
		return
	}
	for _, line := range lines {
		buf.AppendLine(line)
	}
}

func CreateLogWriters(outFile *os.File, errFile *os.File, useUI bool, emitHuman bool) (io.Writer, io.Writer) {
	if useUI || !emitHuman {
		return WriterOrNil(outFile), WriterOrNil(errFile)
	}
	return io.MultiWriter(WriterOrNil(outFile), os.Stdout), io.MultiWriter(WriterOrNil(errFile), os.Stderr)
}

func WriterOrNil(file *os.File) io.Writer {
	if file == nil {
		return nil
	}
	return file
}
