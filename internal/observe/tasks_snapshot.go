package observe

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (o *Observer) loadTaskSnapshot(ctx context.Context, query TaskSummaryQuery) (taskSnapshot, error) {
	if ctx == nil {
		return taskSnapshot{}, errors.New("observe: task summary context is required")
	}
	if err := query.Validate(); err != nil {
		return taskSnapshot{}, err
	}

	tasks, err := o.registry.ListTasks(ctx, taskpkg.Query{
		Scope:       query.Scope,
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
		OwnerKind:   query.OwnerKind.Normalize(),
		OwnerRef:    strings.TrimSpace(query.OwnerRef),
		Search:      strings.TrimSpace(query.Search),
	})
	if err != nil {
		return taskSnapshot{}, fmt.Errorf("observe: list tasks for summary: %w", err)
	}
	tasks = filterTasksByOrigin(tasks, query.OriginKind)
	dependencyCounts, err := o.loadTaskDependencyCounts(ctx, tasks)
	if err != nil {
		return taskSnapshot{}, err
	}
	for idx := range tasks {
		taskID := strings.TrimSpace(tasks[idx].ID)
		if taskID == "" {
			continue
		}
		tasks[idx].DependencyCount = taskpkg.ClampSummaryCount(dependencyCounts[taskID])
	}

	runs, err := o.registry.ListTaskRuns(ctx, taskpkg.RunQuery{})
	if err != nil {
		return taskSnapshot{}, fmt.Errorf("observe: list task runs for summary: %w", err)
	}
	tasksByID, taskIDs := taskSummaryIndex(tasks)
	taskChannels := taskParticipationChannels(tasks, runs)
	tasks = filterTasksByNetworkChannel(tasks, taskChannels, query.ParticipationChannel)

	runsByID := make(map[string]taskpkg.Run, len(runs))
	for _, item := range runs {
		if _, ok := taskIDs[strings.TrimSpace(item.TaskID)]; !ok {
			continue
		}
		runID := strings.TrimSpace(item.ID)
		if runID == "" {
			continue
		}
		runsByID[runID] = item
	}
	runs = filterRuns(runs, taskIDs, query)

	events, err := o.registry.ListTaskEvents(ctx, taskpkg.EventQuery{})
	if err != nil {
		return taskSnapshot{}, fmt.Errorf("observe: list task events for summary: %w", err)
	}
	events = filterEventsForTasks(events, taskIDs)

	workspaceID := strings.TrimSpace(query.WorkspaceID)
	audits, err := o.registry.ListNetworkAudit(ctx, store.NetworkAuditQuery{
		WorkspaceID: workspaceID,
		Global:      workspaceID == "",
		Channel:     strings.TrimSpace(query.ParticipationChannel),
	})
	if err != nil {
		return taskSnapshot{}, fmt.Errorf("observe: list network audit for summary: %w", err)
	}

	return taskSnapshot{
		tasks:        tasks,
		runs:         runs,
		events:       events,
		audits:       audits,
		tasksByID:    tasksByID,
		runsByID:     runsByID,
		taskChannels: taskChannels,
	}, nil
}

func (o *Observer) loadTaskDependencyCounts(
	ctx context.Context,
	tasks []taskpkg.Summary,
) (map[string]int, error) {
	taskIDs := make([]string, 0, len(tasks))
	seen := make(map[string]struct{}, len(tasks))
	for idx := range tasks {
		item := &tasks[idx]
		taskID := strings.TrimSpace(item.ID)
		if taskID == "" {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		taskIDs = append(taskIDs, taskID)
	}
	if len(taskIDs) == 0 {
		return map[string]int{}, nil
	}

	path := strings.TrimSpace(o.registry.Path())
	if path == "" {
		return o.loadTaskDependencyCountsIndividually(ctx, taskIDs)
	}

	db, err := store.OpenSQLiteDatabase(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("observe: open registry database for dependency counts: %w", err)
	}
	counts, queryErr := queryTaskDependencyCounts(ctx, db, taskIDs)
	closeErr := db.Close()
	if queryErr != nil {
		if closeErr != nil {
			return nil, errors.Join(queryErr, fmt.Errorf("observe: close registry database: %w", closeErr))
		}
		return nil, queryErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("observe: close registry database: %w", closeErr)
	}
	return counts, nil
}

func (o *Observer) loadTaskDependencyCountsIndividually(
	ctx context.Context,
	taskIDs []string,
) (map[string]int, error) {
	counts := make(map[string]int, len(taskIDs))
	for _, taskID := range taskIDs {
		count, err := o.registry.CountDependencies(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("observe: count dependencies for task %q: %w", taskID, err)
		}
		counts[taskID] = count
	}
	return counts, nil
}

func queryTaskDependencyCounts(
	ctx context.Context,
	db *sql.DB,
	taskIDs []string,
) (map[string]int, error) {
	taskIDsJSON, err := json.Marshal(taskIDs)
	if err != nil {
		return nil, fmt.Errorf("observe: encode dependency count task ids: %w", err)
	}

	rows, err := db.QueryContext(
		ctx,
		`SELECT dep.task_id, COUNT(1)
		FROM task_dependencies dep
		JOIN json_each(?) requested ON requested.value = dep.task_id
		GROUP BY dep.task_id`,
		string(taskIDsJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("observe: query dependency counts: %w", err)
	}
	defer func() {
		// Scan and iteration errors own the query result; Close only releases an already-consumed read cursor.
		_ = rows.Close()
	}()

	counts := make(map[string]int, len(taskIDs))
	for rows.Next() {
		var taskID string
		var count int
		if err := rows.Scan(&taskID, &count); err != nil {
			return nil, fmt.Errorf("observe: scan dependency counts: %w", err)
		}
		counts[strings.TrimSpace(taskID)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("observe: iterate dependency counts: %w", err)
	}
	return counts, nil
}
