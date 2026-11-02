package globaldb

import "fmt"

// sessionAutomationFamilyRootExpression names the session a row's family hangs from.
const sessionAutomationFamilyRootExpression = `COALESCE(NULLIF(sessions.root_session_id, ''), sessions.id)`

// sessionCatalogFinishedAutomationExpression selects a settled automation-run
// family: an automation job or trigger ran its root session directly, the run
// did not fail, and every hierarchy member is plainly stopped with no operator
// prompt. Failed, waiting and resumed members keep the whole family visible, so
// a visible parent never loses its children. Loop and Task sessions share the
// automation actor but stay out: their run row names the delegated owner and no
// session. Forks and continuations are new top-level sessions, not members.
//
// Existing rows need no backfill because automation_runs already records the
// link. Non-automation rows stop at the initiator comparison.
var sessionCatalogFinishedAutomationExpression = fmt.Sprintf(`(sessions.initiator_kind = 'automation'
 AND (%[1]s = sessions.id OR %[2]s)
 AND EXISTS (SELECT 1 FROM automation_runs AS run
 WHERE run.session_id = %[1]s AND run.status IN ('running', 'completed', 'canceled')
 AND run.loop_run_id IS NULL AND COALESCE(run.task_id, '') = '' AND COALESCE(run.task_run_id, '') = '')
 AND NOT EXISTS (SELECT 1 FROM sessions AS family
 WHERE (family.id = %[1]s OR (family.root_session_id = %[1]s AND %[2]s))
 AND (%[3]s <> 'stopped' OR EXISTS (SELECT 1 FROM session_prompt_admissions AS operator_prompt
 WHERE operator_prompt.workspace_id = family.workspace_id AND operator_prompt.session_id = family.id
 AND operator_prompt.human_ingress = 1))))`,
	sessionAutomationFamilyRootExpression,
	sessionCatalogHierarchyChildExpression,
	sessionCatalogBadgeExpression,
)
