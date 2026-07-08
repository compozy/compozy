package runshared

import "time"

// Clock abstracts the time source so idle windows and tickers advance under test
// control instead of wall time. Production wiring uses RealClock; deterministic
// tests inject a fake implementation.
type Clock interface {
	Now() time.Time
	NewTicker(d time.Duration) Ticker
}

// Ticker abstracts *time.Ticker so a fake clock can drive ticks deterministically.
type Ticker interface {
	// C returns the tick delivery channel.
	C() <-chan time.Time
	// Stop halts the ticker; further ticks are not delivered.
	Stop()
}

// RealClock is the production Clock backed by the standard library.
type RealClock struct{}

var _ Clock = RealClock{}

// Now reports the current wall-clock time.
func (RealClock) Now() time.Time { return time.Now() }

// NewTicker returns a Ticker backed by time.NewTicker.
func (RealClock) NewTicker(d time.Duration) Ticker {
	return &realTicker{ticker: time.NewTicker(d)}
}

type realTicker struct {
	ticker *time.Ticker
}

var _ Ticker = (*realTicker)(nil)

func (r *realTicker) C() <-chan time.Time { return r.ticker.C }

func (r *realTicker) Stop() { r.ticker.Stop() }
