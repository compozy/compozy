package subprocess

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// HealthCheckResponse is the structured result of the health_check RPC.
type HealthCheckResponse struct {
	Healthy bool            `json:"healthy"`
	Message string          `json:"message,omitempty"`
	Details json.RawMessage `json:"details,omitempty"`
}

// HealthState captures the current health-monitor snapshot for the subprocess.
type HealthState struct {
	Healthy             bool
	Message             string
	Details             json.RawMessage
	LastCheckedAt       time.Time
	ConsecutiveFailures int
	LastError           string
}

// CloneHealthState returns an immutable copy of a subprocess health snapshot.
func CloneHealthState(state HealthState) HealthState {
	state.Details = append(json.RawMessage(nil), state.Details...)
	return state
}

// HealthFailureDetected reports whether a snapshot contains an observed failed verdict.
// A zero-value snapshot is unsupported or not yet checked and does not degrade consumers.
func HealthFailureDetected(state HealthState) bool {
	if state.Healthy {
		return false
	}
	return !state.LastCheckedAt.IsZero() ||
		state.ConsecutiveFailures > 0 ||
		strings.TrimSpace(state.LastError) != "" ||
		strings.TrimSpace(state.Message) != "" ||
		len(state.Details) > 0
}

// HealthFailureReason returns the human-readable reason carried by a failed snapshot.
func HealthFailureReason(state HealthState) string {
	parts := make([]string, 0, 2)
	if message := strings.TrimSpace(state.Message); message != "" {
		parts = append(parts, message)
	}
	if lastError := strings.TrimSpace(state.LastError); lastError != "" {
		parts = append(parts, lastError)
	}
	return strings.Join(parts, ": ")
}

type healthMonitor struct {
	mu     sync.RWMutex
	state  HealthState
	active *healthMonitorRun
}

type healthMonitorRun struct {
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

func newHealthMonitorRun() *healthMonitorRun {
	return &healthMonitorRun{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (r *healthMonitorRun) stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		close(r.stopCh)
	})
}

func (r *healthMonitorRun) finish() {
	if r == nil {
		return
	}
	close(r.doneCh)
}

// HealthState returns the latest health-monitor snapshot.
func (p *Process) HealthState() HealthState {
	if p == nil {
		return HealthState{}
	}
	p.health.mu.RLock()
	defer p.health.mu.RUnlock()
	return CloneHealthState(p.health.state)
}

func (p *Process) maybeStartHealthMonitor(runtime InitializeRuntime, supports InitializeSupports) {
	if !supports.HealthCheck || runtime.HealthCheckIntervalMS <= 0 || runtime.HealthCheckTimeoutMS <= 0 {
		return
	}

	p.health.mu.Lock()
	if p.health.active != nil {
		p.health.mu.Unlock()
		return
	}

	run := newHealthMonitorRun()
	p.health.active = run
	p.health.state = HealthState{Healthy: true}
	p.health.mu.Unlock()

	interval := time.Duration(runtime.HealthCheckIntervalMS) * time.Millisecond
	timeout := time.Duration(runtime.HealthCheckTimeoutMS) * time.Millisecond
	go p.runHealthMonitor(run, interval, timeout)
}

func (p *Process) runHealthMonitor(run *healthMonitorRun, interval, timeout time.Duration) {
	defer run.finish()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.lifecycleCtx.Done():
			return
		case <-run.stopCh:
			return
		case <-ticker.C:
			p.runHealthProbe(timeout)
		}
	}
}

func (p *Process) runHealthProbe(timeout time.Duration) {
	probeBaseCtx := context.Background()
	if p != nil && p.lifecycleCtx != nil {
		probeBaseCtx = p.lifecycleCtx
	}
	probeCtx, cancel := context.WithTimeout(probeBaseCtx, timeout)
	defer cancel()

	var response HealthCheckResponse
	err := p.Call(probeCtx, "health_check", struct{}{}, &response)
	if err != nil {
		p.recordHealthFailure(fmt.Errorf("health_check: %w", err))
		return
	}
	if !response.Healthy {
		p.markUnhealthy(response.Message, response.Details, "")
		return
	}
	p.recordHealthy(response)
}

func (p *Process) recordHealthy(response HealthCheckResponse) {
	p.health.mu.Lock()
	defer p.health.mu.Unlock()
	p.health.state = HealthState{
		Healthy:             true,
		Message:             response.Message,
		Details:             append(json.RawMessage(nil), response.Details...),
		LastCheckedAt:       time.Now().UTC(),
		ConsecutiveFailures: 0,
	}
}

func (p *Process) recordHealthFailure(err error) {
	p.health.mu.Lock()
	defer p.health.mu.Unlock()

	state := p.health.state
	state.LastCheckedAt = time.Now().UTC()
	state.ConsecutiveFailures++
	state.LastError = err.Error()
	if state.ConsecutiveFailures >= p.healthThreshold {
		state.Healthy = false
	}
	p.health.state = state
}

func (p *Process) markUnhealthy(message string, details json.RawMessage, lastError string) {
	p.health.mu.Lock()
	defer p.health.mu.Unlock()

	consecutiveFailures := p.health.state.ConsecutiveFailures + 1
	p.health.state = HealthState{
		Healthy:             false,
		Message:             message,
		Details:             append(json.RawMessage(nil), details...),
		LastCheckedAt:       time.Now().UTC(),
		ConsecutiveFailures: consecutiveFailures,
		LastError:           lastError,
	}
}

func (p *Process) stopHealthMonitor() {
	p.health.mu.Lock()
	run := p.health.active
	p.health.active = nil
	p.health.mu.Unlock()

	if run == nil {
		return
	}
	run.stop()
	<-run.doneCh
}
