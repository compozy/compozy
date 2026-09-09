package logger

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

const FailureWarningInterval = 5 * time.Minute

// FailureSample is one bounded observation from a deterministic scan.
type FailureSample struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// FailureSummary retains a bounded sample while fingerprinting every failure in scan order.
type FailureSummary struct {
	Count   int
	Samples []FailureSample
	digest  [sha256.Size]byte
}

func (s *FailureSummary) Add(id string, err error) {
	message := err.Error()
	s.digest = sha256.Sum256(fmt.Appendf(s.digest[:], "%d:%s%d:%s", len(id), id, len(message), message))
	s.Count++
	if len(s.Samples) < 5 {
		s.Samples = append(s.Samples, FailureSample{
			ID: diagnostics.RedactAndBound(id, 256), Error: diagnostics.RedactAndBound(message, 512),
		})
	}
}

// FailureWarnings owns constant-space warning backoff for one scan source.
type FailureWarnings struct {
	mu       sync.Mutex
	digest   [sha256.Size]byte
	lastWarn time.Time
}

func (w *FailureWarnings) Warn(log *slog.Logger, message string, failures FailureSummary, now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if failures.Count == 0 {
		w.lastWarn = time.Time{}
		w.digest = [sha256.Size]byte{}
		return
	}
	if failures.digest == w.digest && !w.lastWarn.IsZero() && now.Sub(w.lastWarn) < FailureWarningInterval {
		return
	}
	w.digest = failures.digest
	w.lastWarn = now
	log.Warn(message, "unreadable_count", failures.Count, "samples", failures.Samples,
		"repeat_interval", FailureWarningInterval, "diagnostic", "compozy doctor")
}
