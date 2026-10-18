package daemon

import (
	"context"
	"sync"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	// nativeGoalAvailabilityTTL backstops Goal transitions that do not advance the projection epoch.
	nativeGoalAvailabilityTTL = 2 * time.Second
	// nativeGoalAvailabilityCacheLimit bounds the per-session cache; overflowing it starts over.
	nativeGoalAvailabilityCacheLimit = 1024
)

// nativeGoalVisibilityProbe answers whether a session has a visible Goal without hydrating it.
type nativeGoalVisibilityProbe interface {
	sessionGoalVisible(ctx context.Context, workspaceID string, sessionID string) (bool, error)
}

var _ nativeGoalVisibilityProbe = (*daemonLoopAPIService)(nil)

type nativeGoalAvailabilityKind uint8

const (
	nativeGoalAvailabilityGet nativeGoalAvailabilityKind = iota + 1
	nativeGoalAvailabilityReport
)

type nativeGoalAvailabilityKey struct {
	kind        nativeGoalAvailabilityKind
	workspaceID string
	sessionID   string
}

type nativeGoalAvailabilityEntry struct {
	epoch     uint64
	checkedAt time.Time
	// reason is empty when the tool is available.
	reason toolspkg.ReasonCode
}

// nativeGoalAvailabilityCache memoizes per-session Goal tool availability. Every projection build
// asks for it, so entries live until the daemon projection epoch advances (Goal commands, Goal
// prompt transitions, and Loop start/terminal all advance it) or the TTL backstop expires.
type nativeGoalAvailabilityCache struct {
	mu      sync.Mutex
	epoch   func() uint64
	now     func() time.Time
	entries map[nativeGoalAvailabilityKey]nativeGoalAvailabilityEntry
}

// newNativeGoalAvailabilityCache returns nil without an epoch so availability stays uncached.
func newNativeGoalAvailabilityCache(epoch func() uint64, now func() time.Time) *nativeGoalAvailabilityCache {
	if epoch == nil {
		return nil
	}
	if now == nil {
		now = time.Now
	}
	return &nativeGoalAvailabilityCache{
		epoch:   epoch,
		now:     now,
		entries: make(map[nativeGoalAvailabilityKey]nativeGoalAvailabilityEntry),
	}
}

// resolve returns the cached decision or computes one; compute reports whether its answer is a
// durable Goal-state decision worth caching (backend failures are always recomputed).
func (c *nativeGoalAvailabilityCache) resolve(
	key nativeGoalAvailabilityKey,
	compute func() (toolspkg.ReasonCode, bool),
) toolspkg.Availability {
	if c == nil {
		reason, _ := compute()
		return nativeGoalAvailability(reason)
	}
	epoch := c.epoch()
	now := c.now()
	c.mu.Lock()
	entry, ok := c.entries[key]
	c.mu.Unlock()
	if ok && entry.epoch == epoch && !now.Before(entry.checkedAt) &&
		now.Sub(entry.checkedAt) < nativeGoalAvailabilityTTL {
		return nativeGoalAvailability(entry.reason)
	}
	reason, cacheable := compute()
	c.mu.Lock()
	if !cacheable {
		delete(c.entries, key)
	} else {
		if len(c.entries) >= nativeGoalAvailabilityCacheLimit {
			clear(c.entries)
		}
		// The epoch captured before compute keeps a racing transition from being masked.
		c.entries[key] = nativeGoalAvailabilityEntry{epoch: epoch, checkedAt: now, reason: reason}
	}
	c.mu.Unlock()
	return nativeGoalAvailability(reason)
}

func nativeGoalAvailability(reason toolspkg.ReasonCode) toolspkg.Availability {
	if reason == "" {
		return toolspkg.Available()
	}
	return toolspkg.Unavailable(reason)
}

func (n *daemonNativeTools) goalGetAvailability(
	ctx context.Context,
	scope toolspkg.Scope,
) toolspkg.Availability {
	if n.loopService() == nil {
		return toolspkg.Unavailable(toolspkg.ReasonDependencyMissing)
	}
	workspaceID, sessionID, ok := nativeGoalScope(scope)
	if !ok {
		return toolspkg.Unavailable(toolspkg.ReasonSessionDenied)
	}
	key := nativeGoalAvailabilityKey{
		kind: nativeGoalAvailabilityGet, workspaceID: workspaceID, sessionID: sessionID,
	}
	return n.goalAvailability.resolve(key, func() (toolspkg.ReasonCode, bool) {
		visible, err := n.sessionGoalVisible(ctx, workspaceID, sessionID)
		switch {
		case err != nil && nativeGoalNotActive(err):
			return toolspkg.ReasonGoalNotActive, true
		case err != nil:
			return toolspkg.ReasonBackendUnhealthy, false
		case !visible:
			return toolspkg.ReasonGoalNotActive, true
		default:
			return "", true
		}
	})
}

func (n *daemonNativeTools) goalReportAvailability(
	ctx context.Context,
	scope toolspkg.Scope,
) toolspkg.Availability {
	runtime := n.goalRuntime()
	if runtime == nil {
		return toolspkg.Unavailable(toolspkg.ReasonDependencyMissing)
	}
	workspaceID, sessionID, ok := nativeGoalScope(scope)
	if !ok {
		return toolspkg.Unavailable(toolspkg.ReasonSessionDenied)
	}
	key := nativeGoalAvailabilityKey{
		kind: nativeGoalAvailabilityReport, workspaceID: workspaceID, sessionID: sessionID,
	}
	return n.goalAvailability.resolve(key, func() (toolspkg.ReasonCode, bool) {
		_, found, err := runtime.findGoalReportTarget(ctx, looppkg.WorkspaceID(workspaceID), sessionID)
		switch {
		case err != nil:
			return toolspkg.ReasonBackendUnhealthy, false
		case !found:
			return toolspkg.ReasonGoalNotActive, true
		default:
			return "", true
		}
	})
}

// sessionGoalVisible mirrors visibleGoalSnapshot != nil, including active moved-binding aliases,
// using the hydration-free probe when the Loop service offers one.
func (n *daemonNativeTools) sessionGoalVisible(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (bool, error) {
	probe, ok := n.loopService().(nativeGoalVisibilityProbe)
	if !ok {
		snapshot, err := n.visibleGoalSnapshot(ctx, workspaceID, sessionID)
		return snapshot != nil, err
	}
	visible, err := probe.sessionGoalVisible(ctx, workspaceID, sessionID)
	if err != nil || visible {
		return visible, err
	}
	runtime := n.goalRuntime()
	if runtime == nil {
		return false, nil
	}
	originSessionID, found, err := runtime.resolveActiveGoalOriginAlias(
		ctx,
		looppkg.WorkspaceID(workspaceID),
		sessionID,
	)
	if err != nil || !found {
		return false, err
	}
	return probe.sessionGoalVisible(ctx, workspaceID, originSessionID)
}
