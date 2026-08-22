package core

import (
	"strings"

	taskpkg "github.com/compozy/compozy/internal/task"
)

const loopCoordinatorCreatedByRef = "loop-coordinator"

// ApplyTaskLoopCatalogFilters maps public Loop filters into the neutral catalog query.
func ApplyTaskLoopCatalogFilters(query *taskpkg.CatalogQuery, includeLoop bool, loopRunID string) {
	if query == nil {
		return
	}
	query.LoopRunID = strings.TrimSpace(loopRunID)
	if includeLoop || query.LoopRunID != "" || strings.TrimSpace(query.ParentTaskID) != "" {
		query.ExcludeCreatedBy = nil
		return
	}
	query.ExcludeCreatedBy = defaultTaskLoopExclusions()
}

func defaultTaskLoopExclusions() []taskpkg.ActorRef {
	return []taskpkg.ActorRef{{
		Kind: taskpkg.ActorKindDaemon,
		Ref:  loopCoordinatorCreatedByRef,
	}}
}
