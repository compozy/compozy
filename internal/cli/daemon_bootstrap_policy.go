package cli

import (
	"errors"
	"sync"
	"time"
)

const (
	bootstrapMaximumAttempts = 5
	bootstrapInitialBackoff  = 500 * time.Millisecond
	bootstrapMaximumBackoff  = 10 * time.Second
)

type bootstrapResolution string

const (
	bootstrapResolutionAttach    bootstrapResolution = "attach"
	bootstrapResolutionStart     bootstrapResolution = "start"
	bootstrapResolutionProvision bootstrapResolution = "provision"
)

type bootstrapProbeClass string

const (
	bootstrapProbeHealthy            bootstrapProbeClass = "healthy"
	bootstrapProbeListeningUnhealthy bootstrapProbeClass = "listening-unhealthy"
	bootstrapProbeStaleRecord        bootstrapProbeClass = "stale-record"
	bootstrapProbePortConflict       bootstrapProbeClass = "port-conflict"
	bootstrapProbeUnavailable        bootstrapProbeClass = "unavailable"
)

type bootstrapProbe struct {
	Healthy       bool
	Listening     bool
	RecordPresent bool
	RecordMatches bool
	PortOccupied  bool
	Installed     bool
}

func classifyBootstrapProbe(probe bootstrapProbe) bootstrapProbeClass {
	switch {
	case probe.Healthy:
		return bootstrapProbeHealthy
	case probe.Listening:
		return bootstrapProbeListeningUnhealthy
	case probe.RecordPresent && !probe.RecordMatches:
		return bootstrapProbeStaleRecord
	case probe.PortOccupied:
		return bootstrapProbePortConflict
	default:
		return bootstrapProbeUnavailable
	}
}

func resolveBootstrapAction(probe bootstrapProbe) bootstrapResolution {
	if probe.Healthy {
		return bootstrapResolutionAttach
	}
	if probe.Installed {
		return bootstrapResolutionStart
	}
	return bootstrapResolutionProvision
}

func bootstrapBackoff(attempt int) time.Duration {
	if attempt < 1 {
		return 0
	}
	delay := bootstrapInitialBackoff
	for current := 1; current < attempt; current++ {
		delay *= 2
		if delay >= bootstrapMaximumBackoff {
			return bootstrapMaximumBackoff
		}
	}
	return delay
}

func bootstrapShouldGiveUp(unreadiedStarts int) bool {
	return unreadiedStarts >= bootstrapMaximumAttempts
}

type bootstrapAttemptGate struct {
	mu       sync.Mutex
	inFlight bool
}

var errBootstrapAttemptInFlight = errors.New("cli: daemon bootstrap attempt already in flight")

func (g *bootstrapAttemptGate) Begin() (func(), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inFlight {
		return nil, errBootstrapAttemptInFlight
	}
	g.inFlight = true
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			g.inFlight = false
			g.mu.Unlock()
		})
	}, nil
}
