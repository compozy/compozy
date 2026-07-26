package daemon

import (
	"context"
	"strings"
	"time"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

type soulRunStore interface {
	ListTaskRuns(ctx context.Context, query taskpkg.RunQuery) ([]taskpkg.Run, error)
}

type soulRunActivityChecker struct {
	store soulRunStore
}

func soulSnapshotStoreDependency(value any) session.SoulSnapshotStore {
	store, ok := value.(session.SoulSnapshotStore)
	if !ok {
		return nil
	}
	return store
}

func sessionHealthStoreDependency(value any) session.HealthStore {
	store, ok := value.(session.HealthStore)
	if !ok {
		return nil
	}
	return store
}

func soulRunActivityCheckerDependency(value any) session.SoulRunActivityChecker {
	store, ok := value.(soulRunStore)
	if !ok {
		return nil
	}
	return soulRunActivityChecker{store: store}
}

func (c soulRunActivityChecker) HasActiveRunForSession(
	ctx context.Context,
	sessionID string,
	now time.Time,
) (bool, error) {
	if c.store == nil || strings.TrimSpace(sessionID) == "" {
		return false, nil
	}
	runs, err := c.store.ListTaskRuns(ctx, taskpkg.RunQuery{SessionID: strings.TrimSpace(sessionID)})
	if err != nil {
		return false, err
	}
	now = now.UTC()
	for _, run := range runs {
		if strings.TrimSpace(run.SessionID) != strings.TrimSpace(sessionID) {
			continue
		}
		switch run.Status.Normalize() {
		case taskpkg.TaskRunStatusClaimed, taskpkg.TaskRunStatusStarting, taskpkg.TaskRunStatusRunning:
			if run.LeaseUntil.IsZero() || run.LeaseUntil.After(now) {
				return true, nil
			}
		default:
		}
	}
	return false, nil
}
