package testutil

import (
	"context"

	taskpkg "github.com/compozy/compozy/internal/task"
)

// ListTasks exposes the unpaged task-list seam used by internal lifecycle tests.
func (s *StubTaskManager) ListTasks(
	ctx context.Context,
	query taskpkg.Query,
	actor taskpkg.ActorContext,
) ([]taskpkg.Summary, error) {
	if s.ListTasksFn != nil {
		return s.ListTasksFn(ctx, query, actor)
	}
	return nil, nil
}

// ListTaskCatalog exposes the bounded catalog seam through the most specific configured callback.
func (s *StubTaskManager) ListTaskCatalog(
	ctx context.Context,
	query taskpkg.CatalogQuery,
	actor taskpkg.ActorContext,
) (taskpkg.CatalogPage, error) {
	if s.ListTaskCatalogFn != nil {
		return s.ListTaskCatalogFn(ctx, query, actor)
	}
	if s.ListTasksFn == nil {
		return taskpkg.CatalogPage{Limit: query.Limit}, nil
	}
	limit := query.Limit

	tasks, err := s.ListTasksFn(ctx, taskpkg.Query{
		ReadScope:     query.ReadScope,
		Scope:         taskpkg.Scope(query.Scope),
		WorkspaceID:   query.WorkspaceID,
		Status:        query.Status,
		Priority:      query.Priority,
		ApprovalState: query.ApprovalState,
		OwnerKind:     query.OwnerKind,
		OwnerRef:      query.OwnerRef,
		ParentTaskID:  query.ParentTaskID,
		Search:        query.Search,
		Limit:         limit,
	}, actor)
	if err != nil {
		return taskpkg.CatalogPage{}, err
	}
	total := len(tasks)
	page := taskpkg.CatalogPage{Tasks: tasks, Total: total, Limit: query.Limit}
	if query.Limit <= 0 || len(page.Tasks) <= query.Limit {
		return page, nil
	}
	page.HasMore = true
	page.Tasks = page.Tasks[:query.Limit]
	page.NextCursor, err = taskpkg.EncodeCatalogCursor(query, &page.Tasks[len(page.Tasks)-1])
	if err != nil {
		return taskpkg.CatalogPage{}, err
	}
	return page, nil
}
