package mcp

import (
	"context"
	"strconv"
	"sync/atomic"

	"github.com/compozy/compozy/internal/tools"
)

// ProjectionEpoch is the daemon-owned monotonic invalidation source for hosted tool projections.
type ProjectionEpoch struct {
	value atomic.Uint64
}

// NewProjectionEpoch creates an initialized projection epoch.
func NewProjectionEpoch() *ProjectionEpoch {
	epoch := &ProjectionEpoch{}
	epoch.value.Store(1)
	return epoch
}

// Snapshot returns the current authoritative daemon projection epoch.
func (e *ProjectionEpoch) Snapshot(_ context.Context, _ tools.Scope) (string, bool) {
	if e == nil {
		return "", false
	}
	return strconv.FormatUint(e.value.Load(), 10), true
}

// Current returns the current epoch without changing it.
func (e *ProjectionEpoch) Current() uint64 {
	if e == nil {
		return 0
	}
	return e.value.Load()
}

// Advance invalidates projections built against an earlier daemon state.
func (e *ProjectionEpoch) Advance() uint64 {
	if e == nil {
		return 0
	}
	return e.value.Add(1)
}
