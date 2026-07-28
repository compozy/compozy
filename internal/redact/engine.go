package redact

import (
	"sync"
	"sync/atomic"
)

// Options configures one independent redaction engine.
type Options struct {
	// Disabled turns off additive provider-prefix and entropy heuristics. Exact
	// claim, assignment, reference, and registered-secret protection remains active.
	Disabled bool
}

// Engine applies exact protections and optional conservative heuristics.
type Engine struct {
	heuristicsEnabled bool
}

var (
	processEngine   atomic.Pointer[Engine]
	processSnapshot sync.Once
)

// New constructs an independent redaction engine.
func New(opts Options) *Engine {
	return &Engine{heuristicsEnabled: !opts.Disabled}
}

// RedactString removes secret material from free text.
func (e *Engine) RedactString(value string) string {
	redacted := exactRedactString(value)
	if e == nil || !e.heuristicsEnabled {
		return redacted
	}
	for _, pattern := range heuristicProviderTokenPatterns {
		redacted = pattern.ReplaceAllString(redacted, Marker)
	}
	return redactEntropyCandidates(redacted)
}

// SnapshotEnabled freezes the process-wide additive heuristic setting. Later
// calls cannot change it; configuration changes therefore require a restart.
func SnapshotEnabled(enabled bool) {
	processSnapshot.Do(func() {
		processEngine.Store(New(Options{Disabled: !enabled}))
	})
}

// Enabled reports the process-snapshotted additive heuristic setting.
func Enabled() bool {
	return processRedactor().heuristicsEnabled
}

func processRedactor() *Engine {
	engine := processEngine.Load()
	if engine != nil {
		return engine
	}
	return New(Options{})
}
