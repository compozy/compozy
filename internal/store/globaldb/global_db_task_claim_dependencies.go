package globaldb

import taskpkg "github.com/compozy/agh/internal/task"

const unresolvedTaskDependencyClaimPredicate = `NOT EXISTS (
	SELECT 1
	  FROM task_dependencies dependency
	  JOIN tasks prerequisite ON prerequisite.id = dependency.depends_on_task_id
	  LEFT JOIN task_runs latest_terminal ON latest_terminal.id = (
		SELECT candidate.id
		  FROM task_runs candidate
		 WHERE candidate.task_id = prerequisite.id
		   AND candidate.status IN (?, ?, ?)
		 ORDER BY candidate.attempt DESC, candidate.queued_at DESC, candidate.id DESC
		 LIMIT 1
	  )
	 WHERE dependency.task_id = t.id
	   AND dependency.kind = ?
	   AND CASE
		WHEN prerequisite.status IN (?, ?) THEN 1
		WHEN EXISTS (
			SELECT 1 FROM task_runs active_run
			 WHERE active_run.task_id = prerequisite.id AND active_run.status IN (?, ?)
		) THEN 1
		WHEN EXISTS (
			SELECT 1 FROM task_runs queued_run
			 WHERE queued_run.task_id = prerequisite.id AND queued_run.status IN (?, ?)
		) THEN 1
		WHEN latest_terminal.status = ? THEN 0
		WHEN latest_terminal.status = ? THEN 1
		WHEN latest_terminal.status = ?
		 AND COALESCE((
			SELECT MAX(attempt) FROM task_runs run_attempt
			 WHERE run_attempt.task_id = prerequisite.id
		 ), 0) + 1 > prerequisite.max_attempts THEN 1
		WHEN prerequisite.status = ? THEN 0
		ELSE 1
	   END = 1
)`

func unresolvedTaskDependencyClaimArgs() []any {
	return []any{
		taskpkg.TaskRunStatusCompleted.String(),
		taskpkg.TaskRunStatusFailed.String(),
		taskpkg.TaskRunStatusCanceled.String(),
		string(taskpkg.DependencyKindBlocks),
		string(taskpkg.TaskStatusCanceled),
		string(taskpkg.TaskStatusDraft),
		taskpkg.TaskRunStatusStarting.String(),
		taskpkg.TaskRunStatusRunning.String(),
		taskpkg.TaskRunStatusQueued.String(),
		taskpkg.TaskRunStatusClaimed.String(),
		taskpkg.TaskRunStatusCompleted.String(),
		taskpkg.TaskRunStatusCanceled.String(),
		taskpkg.TaskRunStatusFailed.String(),
		string(taskpkg.TaskStatusCompleted),
	}
}
