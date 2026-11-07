package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// sessionCatalogAutomationRunExpression selects an automation job or trigger
// run family in any state: running, waiting, failed or finished. The root is
// recognized from the moment its row exists, before the dispatcher records the
// run's session link, because the dispatcher creates the session with the job
// or trigger as its initiator and hierarchy children inherit that initiator. The
// run link keeps finished runs hidden after their definition is deleted. A
// family the operator prompted has become the operator's work and stays visible.
// Loop and Task sessions share the automation actor but name a loop run or a
// task as initiator, so they never match. Forks and continuations are new
// top-level sessions, not members.
var sessionCatalogAutomationRunExpression = fmt.Sprintf(`(sessions.initiator_kind = 'automation'
 AND (%[1]s = sessions.id OR %[2]s)
 AND (EXISTS (SELECT 1 FROM automation_jobs AS job WHERE job.id = sessions.initiator_id)
 OR EXISTS (SELECT 1 FROM automation_triggers AS trigger_definition
 WHERE trigger_definition.id = sessions.initiator_id)
 OR EXISTS (SELECT 1 FROM automation_runs AS run
 WHERE run.session_id = %[1]s AND run.loop_run_id IS NULL
 AND COALESCE(run.task_id, '') = '' AND COALESCE(run.task_run_id, '') = ''))
 AND NOT EXISTS (SELECT 1 FROM sessions AS family
 WHERE (family.id = %[1]s OR (family.root_session_id = %[1]s AND %[2]s))
 AND EXISTS (SELECT 1 FROM session_prompt_admissions AS operator_prompt
 WHERE operator_prompt.workspace_id = family.workspace_id AND operator_prompt.session_id = family.id
 AND operator_prompt.human_ingress = 1)))`,
	sessionAutomationFamilyRootExpression,
	sessionCatalogHierarchyChildExpression,
)

// AutomationRunSessionIDs returns the given sessions that the ExcludeAutomation
// catalog rule hides, so live rows overlaid on a page follow the durable rule.
func (g *SessionRepo) AutomationRunSessionIDs(ctx context.Context, sessionIDs []string) (ids []string, err error) {
	if err := g.checkReady(ctx, "automation run sessions"); err != nil {
		return nil, err
	}
	candidates := make([]string, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			candidates = append(candidates, trimmed)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(candidates)
	if err != nil {
		return nil, fmt.Errorf("store: encode automation run session ids: %w", err)
	}
	// dynamic-sql: the catalog visibility expression is shared with the page filters.
	statement := store.AppendWhere("SELECT id FROM sessions", []string{
		"id IN (SELECT CAST(value AS TEXT) FROM json_each(?))",
		sessionCatalogAutomationRunExpression,
	})
	rows, err := g.readDB.QueryContext(ctx, statement, string(encoded))
	if err != nil {
		return nil, fmt.Errorf("store: query automation run sessions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close automation run sessions: %w", closeErr))
		}
	}()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: scan automation run session: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate automation run sessions: %w", err)
	}
	return ids, nil
}
