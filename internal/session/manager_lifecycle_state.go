package session

import (
	"context"
	"sync"
)

// sessionCompactionLifecycle owns all mutable state for background compaction runs.
type sessionCompactionLifecycle struct {
	mu      sync.Mutex
	runs    map[string]*sessionCompactionState
	closing bool
}

// sessionStartLifecycle owns all mutable state for sessions still starting.
type sessionStartLifecycle struct {
	mu      sync.Mutex
	runs    map[string]*sessionStartRun
	wg      sync.WaitGroup
	closing bool
}

// sessionResumeLifecycle owns coalesced stopped-session resume runs.
type sessionResumeLifecycle struct {
	mu      sync.Mutex
	runs    map[string]*sessionResumeRun
	closing bool
}

// sessionProcessWatchLifecycle owns the shared context and admitted process watchers.
type sessionProcessWatchLifecycle struct {
	mu      sync.Mutex
	runs    map[*processWatchRun]struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	closing bool
}
