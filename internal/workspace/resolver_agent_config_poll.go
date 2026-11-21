package workspace

import (
	"context"
	"errors"
	"maps"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/filesnap"
)

type agentConfigObservation struct {
	snapshots  map[string]filesnap.Snapshot
	lastAccess time.Time
}

type AgentConfigScope struct {
	WorkspaceRef string
	ProfileName  string
}

// AgentConfigRevision fences policy snapshots across explicit and observed invalidation.
func (r *Resolver) AgentConfigRevision() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agentConfigGeneration
}

// WatchAgentConfigs retains dependencies for cached consumers that may not resolve policy again.
func (r *Resolver) WatchAgentConfigs(ctx context.Context, scopes []AgentConfigScope) error {
	snapshots := make(map[string]map[string]filesnap.Snapshot, len(scopes))
	for _, scope := range scopes {
		profile := strings.TrimSpace(scope.ProfileName)
		if profile != "" {
			var err error
			profile, _, err = r.resolveProfileIdentity(ctx, profile)
			if err != nil {
				return err
			}
		}
		var ws Workspace
		if ParseRef(scope.WorkspaceRef).Kind != RefUser {
			var err error
			ws, _, err = r.resolveRegistration(ctx, scope.WorkspaceRef)
			if errors.Is(err, ErrWorkspaceNotFound) || IsRootUnavailable(err) {
				continue
			}
			if err != nil {
				return err
			}
		}
		key := workspaceProfileCacheKey(ws.ID, profile)
		if _, found := snapshots[key]; found {
			continue
		}
		scan, err := r.scanAgentConfig(ctx, ws, profile)
		if err != nil {
			return err
		}
		snapshots[key] = scan.snapshots
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agentConfigWatched = make(map[string]struct{}, len(snapshots))
	for key, paths := range snapshots {
		r.touchAgentConfigObservationLocked(key, paths, r.now())
		r.agentConfigWatched[key] = struct{}{}
	}
	return nil
}

// A direct read can see an edit before the observer wakes the hosted streams.
func (r *Resolver) touchAgentConfigObservationLocked(
	key string, snapshots map[string]filesnap.Snapshot, now time.Time,
) {
	previous := r.agentConfigObserved[key]
	if previous == nil {
		r.agentConfigObserved[key] = &agentConfigObservation{cloneSnapshots(snapshots), now}
		return
	}
	previous.lastAccess = now
	var added map[string]filesnap.Snapshot
	for path, snapshot := range snapshots {
		if _, found := previous.snapshots[path]; !found {
			if added == nil {
				added = cloneSnapshots(previous.snapshots)
			}
			added[path] = snapshot
		}
	}
	if added != nil {
		r.agentConfigObserved[key] = &agentConfigObservation{added, now}
	}
}

// PollAgentConfig invalidates changed narrow snapshots with stat-only checks, shared across scopes.
// The daemon owns its cadence independently of the configurable skill discovery watcher.
func (r *Resolver) PollAgentConfig(ctx context.Context) (bool, error) {
	validatedAt := r.now()
	r.mu.RLock()
	observed := maps.Clone(r.agentConfigObserved)
	generation := r.agentConfigGeneration
	r.mu.RUnlock()
	current := make(map[string]filesnap.Snapshot)
	changed := false
	var captureErr error
	for _, previous := range observed {
		for path, expected := range previous.snapshots {
			if err := checkContext(ctx); err != nil {
				return false, err
			}
			if _, found := current[path]; !found {
				if err := addDependencySnapshot(path, current); err != nil {
					// An unreadable dependency is one state transition, not a new edit every tick.
					current[path] = filesnap.Snapshot{Size: -2}
					captureErr = errors.Join(captureErr, err)
				}
			}
			changed = changed || !expected.Equal(current[path])
		}
	}
	r.mu.Lock()
	for key, previous := range observed {
		if r.agentConfigObserved[key] != previous {
			continue
		}
		paths := previous.snapshots
		cached := r.agentConfigCache[key]
		if cached != nil {
			paths = cached.snapshots
		}
		next := cloneSnapshots(paths)
		for path := range next {
			if snapshot, found := current[path]; found {
				next[path] = snapshot
			}
		}
		r.agentConfigObserved[key] = &agentConfigObservation{next, previous.lastAccess}
		// Renew only the same dependency set, at the poll's start, retaining the build fence.
		if !changed && captureErr == nil && generation == r.agentConfigGeneration && cached != nil &&
			filesnap.Equal(cached.snapshots, next) && !validatedAt.Before(cached.validatedAt) {
			cached.validatedAt = validatedAt
		}
	}
	r.mu.Unlock()
	if changed {
		r.InvalidateAll()
	}
	return changed, captureErr
}
