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

// sessionAutomationRunCandidatesQuery classifies live sessions. A saved row
// follows the catalog rule. A live session the catalog has not saved yet,
// because a start registers before it persists, is an automation run when its
// initiator names an existing job or trigger and no member of its family has an
// operator prompt, the same exception the saved rule applies.
var sessionAutomationRunCandidatesQuery = fmt.Sprintf(`SELECT candidate.id FROM (
 SELECT CAST(json_extract(value, '$.id') AS TEXT) AS id,
 CAST(json_extract(value, '$.initiator_id') AS TEXT) AS initiator_id,
 CAST(json_extract(value, '$.family_root_id') AS TEXT) AS family_root_id FROM json_each(?)) AS candidate
 WHERE EXISTS (SELECT 1 FROM sessions WHERE sessions.id = candidate.id AND %[1]s)
 OR (NOT EXISTS (SELECT 1 FROM sessions WHERE sessions.id = candidate.id)
 AND (EXISTS (SELECT 1 FROM automation_jobs AS job WHERE job.id = candidate.initiator_id)
 OR EXISTS (SELECT 1 FROM automation_triggers AS trigger_definition
 WHERE trigger_definition.id = candidate.initiator_id))
 AND NOT EXISTS (SELECT 1 FROM sessions AS family
 WHERE (family.id = candidate.family_root_id OR (family.root_session_id = candidate.family_root_id AND %[2]s))
 AND EXISTS (SELECT 1 FROM session_prompt_admissions AS operator_prompt
 WHERE operator_prompt.workspace_id = family.workspace_id AND operator_prompt.session_id = family.id
 AND operator_prompt.human_ingress = 1)))`,
	sessionCatalogAutomationRunExpression,
	sessionCatalogHierarchyChildExpression,
)

// AutomationRunSessionIDs returns the given live sessions that the
// ExcludeAutomation catalog rule hides, so rows overlaid on a page follow it.
func (g *SessionRepo) AutomationRunSessionIDs(
	ctx context.Context,
	candidates []store.SessionAutomationRunCandidate,
) (ids []string, err error) {
	if err := g.checkReady(ctx, "automation run sessions"); err != nil {
		return nil, err
	}
	type candidateJSON struct {
		ID           string `json:"id"`
		InitiatorID  string `json:"initiator_id"`
		FamilyRootID string `json:"family_root_id"`
	}
	values := make([]candidateJSON, 0, len(candidates))
	for _, candidate := range candidates {
		id := strings.TrimSpace(candidate.ID)
		if id == "" {
			continue
		}
		root := strings.TrimSpace(candidate.FamilyRootID)
		if root == "" {
			root = id
		}
		values = append(values, candidateJSON{
			ID:           id,
			InitiatorID:  strings.TrimSpace(candidate.InitiatorID),
			FamilyRootID: root,
		})
	}
	if len(values) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("store: encode automation run session candidates: %w", err)
	}
	// dynamic-sql: the catalog visibility expression is shared with the page filters.
	rows, err := g.readDB.QueryContext(ctx, sessionAutomationRunCandidatesQuery, string(encoded))
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
