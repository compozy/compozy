package store

// -----
// Shared SQL Queries
// -----

// TaskHierarchyCTEQuery is a shared recursive CTE used to fetch all task states in a workflow,
// including nested children, preserving the exact string used across the codebase to satisfy
// goconst duplication rules.
const TaskHierarchyCTEQuery = `
		WITH RECURSIVE task_hierarchy AS (
			-- Base case: top-level tasks
			SELECT *
			FROM task_states
			WHERE workflow_exec_id = $1

			UNION ALL

			-- Recursive case: child tasks at any depth
			SELECT ts.*
			FROM task_states ts
			INNER JOIN task_hierarchy th ON ts.parent_state_id = th.task_exec_id
		)
		SELECT * FROM task_hierarchy
`
