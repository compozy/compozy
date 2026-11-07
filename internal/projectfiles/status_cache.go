package projectfiles

import (
	"context"
	"fmt"
	"time"
)

type statusSnapshot struct {
	states      map[string]GitStatus
	ignored     map[string]bool
	changedDirs map[string]bool
	branch      string
	expires     time.Time
}

func (r *FileReader) cachedStatus(
	ctx context.Context,
	root Root,
	refresh bool,
) (map[string]GitStatus, map[string]bool, map[string]bool, string, error) {
	r.statusMu.Lock()
	now := r.now()
	for key, value := range r.statusCache {
		if !now.Before(value.expires) {
			delete(r.statusCache, key)
		}
	}

	epoch := r.statusEpoch
	cached, ok := r.statusCache[root]
	r.statusMu.Unlock()
	if ok && !refresh {
		return cached.states, cached.ignored, cached.changedDirs, cached.branch, nil
	}
	key := fmt.Sprintf("%#v:%d", root, epoch)
	if refresh {
		key = fmt.Sprintf("%#v:refresh", root)
	}
	result := r.statusFlight.DoChan(key, func() (any, error) {
		if refresh {
			r.statusMu.Lock()
			delete(r.statusCache, root)
			r.statusEpoch++
			epoch = r.statusEpoch
			r.statusMu.Unlock()
		}
		states, ignored, branch, err := r.status(ctx, root)
		if err != nil {
			return nil, err
		}
		snapshot := statusSnapshot{
			states:      states,
			ignored:     ignored,
			changedDirs: changedDirectories(states),
			branch:      branch,
			expires:     r.now().Add(r.statusTTL),
		}
		r.statusMu.Lock()
		if epoch == r.statusEpoch {
			r.statusCache[root] = snapshot
		}
		r.statusMu.Unlock()
		return snapshot, nil
	})
	select {
	case <-ctx.Done():
		return nil, nil, nil, "", ctx.Err()
	case item := <-result:
		if item.Err != nil {
			return nil, nil, nil, "", item.Err
		}
		snapshot, ok := item.Val.(statusSnapshot)
		if !ok {
			return nil, nil, nil, "", ErrRootUnavailable
		}
		return snapshot.states, snapshot.ignored, snapshot.changedDirs, snapshot.branch, nil
	}
}
