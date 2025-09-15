package postgres

// TaskHierarchyCTEQuery returns all task states in a workflow including nested children.
const TaskHierarchyCTEQuery = `
		WITH RECURSIVE task_hierarchy AS (
			-- Base case: top-level tasks only (parent_state_id IS NULL)
			SELECT *
			FROM task_states
			WHERE workflow_exec_id = $1 AND parent_state_id IS NULL

			UNION ALL

			-- Recursive case: child tasks at any depth
			SELECT ts.*
			FROM task_states ts
			INNER JOIN task_hierarchy th ON ts.parent_state_id = th.task_exec_id
			WHERE ts.workflow_exec_id = $1
		)
		SELECT * FROM task_hierarchy
`
