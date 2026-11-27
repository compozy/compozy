-- name: GetAgentInstance :one
SELECT * FROM agent_instances WHERE id = ?;

-- name: FindLiveAgentInstance :one
SELECT * FROM agent_instances WHERE profile_id = ? AND agent_name = ? AND workspace_id = ? AND archived_at IS NULL;

-- name: CountActiveAgentInstances :one
SELECT count(*) FROM agent_instances WHERE profile_id = ? AND archived_at IS NULL AND state IN ('starting', 'active');

-- name: InsertAgentInstance :exec
INSERT INTO agent_instances (id, profile_id, agent_name, workspace_id, state, grant_ceiling, permission_ceiling, depth, created_by_kind, created_by_id, created_at, updated_at)
VALUES (?, ?, ?, ?, 'starting', ?, ?, ?, ?, ?, ?, ?);

-- name: BindAgentInstanceMain :execrows
UPDATE agent_instances SET main_session_id = sqlc.arg(session_id), state = 'active', updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND state = 'starting' AND archived_at IS NULL AND main_session_id IS NULL;

-- name: PauseAgentInstance :execrows
UPDATE agent_instances SET state = 'paused', pause_reason = sqlc.arg(pause_reason), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND archived_at IS NULL
AND (state = 'active' OR (state = 'paused' AND sqlc.arg(pause_reason) = 'manual'
 AND pause_reason IN ('definition_missing', 'definition_not_always_on')));

-- name: QuarantineAgentInstanceDailyBudget :execrows
UPDATE agent_instances SET state='paused', pause_reason='daily_budget_recovery', updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND archived_at IS NULL AND state = 'active';

-- name: RecordAgentInstanceDailyBudgetRecovery :exec
INSERT INTO agent_instance_daily_budget_recovery(instance_id, generation, pending)
VALUES (sqlc.arg(instance_id), 1, 1)
ON CONFLICT(instance_id) DO UPDATE SET generation = generation + 1, pending = 1;

-- name: GetAgentInstanceDailyBudgetRecovery :one
SELECT generation FROM agent_instance_daily_budget_recovery WHERE instance_id = sqlc.arg(instance_id) AND pending = 1;

-- name: ClearAgentInstanceDailyBudgetRecovery :execrows
UPDATE agent_instance_daily_budget_recovery SET pending = 0
WHERE instance_id = sqlc.arg(instance_id) AND generation = sqlc.arg(generation) AND pending = 1;

-- name: RestoreAgentInstanceAfterDailyBudgetRecovery :exec
UPDATE agent_instances SET pause_reason = 'daily_budget', updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND state = 'paused' AND pause_reason = 'daily_budget_recovery' AND archived_at IS NULL;

-- name: LatchAgentInstanceDailyBudgetPause :exec
INSERT INTO agent_instance_daily_budget_recovery(instance_id, generation, pending, latched)
VALUES (sqlc.arg(instance_id), 1, 0, 1)
ON CONFLICT(instance_id) DO UPDATE SET latched = 1;

-- name: CountLatchedAgentInstanceBudgetPauses :one
SELECT count(*) FROM agent_instance_daily_budget_recovery WHERE instance_id = sqlc.arg(instance_id) AND latched = 1;

-- name: ClearLatchedAgentInstanceBudgetPause :exec
UPDATE agent_instance_daily_budget_recovery SET latched = 0 WHERE instance_id = sqlc.arg(instance_id) AND latched = 1;

-- name: CountUncertainDailyBudgetDays :one
SELECT COUNT(*) FROM agent_instance_daily_budget_uncertain_days
WHERE instance_id=sqlc.arg(instance_id) AND day=sqlc.arg(day);

-- name: MarkUncertainDailyBudgetDay :exec
INSERT INTO agent_instance_daily_budget_uncertain_days(instance_id, day, source_session_id, recorded_at)
VALUES (sqlc.arg(instance_id), sqlc.arg(day), sqlc.arg(source_session_id), sqlc.arg(recorded_at))
ON CONFLICT(instance_id, day) DO NOTHING;

-- name: ResumeAgentInstance :execrows
UPDATE agent_instances SET state = 'active', pause_reason = '', updated_at = ?
WHERE id = ? AND state = 'paused' AND archived_at IS NULL;

-- name: ArchiveAgentInstance :execrows
UPDATE agent_instances SET archived_at = ?, updated_at = ? WHERE id = ? AND archived_at IS NULL;

-- name: ArchiveWorkspaceAgentInstances :exec
UPDATE agent_instances SET archived_at = sqlc.arg(archived_at), updated_at = sqlc.arg(updated_at)
WHERE workspace_id = sqlc.arg(workspace_id) AND archived_at IS NULL;

-- name: DeleteUnstartedAgentInstance :execrows
DELETE FROM agent_instances WHERE agent_instances.id = ? AND state = 'starting' AND main_session_id IS NULL
AND NOT EXISTS (SELECT 1 FROM sessions WHERE instance_id = agent_instances.id);

-- name: ListAgentInstances :many
SELECT * FROM agent_instances WHERE (sqlc.arg(all_profiles) = 1 OR profile_id = sqlc.arg(profile_id))
AND (sqlc.arg(all_workspaces) = 1 OR workspace_id = sqlc.arg(workspace_id))
AND (sqlc.arg(agent_name) = '' OR agent_name = sqlc.arg(agent_name))
AND (sqlc.arg(state) = '' OR state = sqlc.arg(state))
AND (sqlc.arg(include_archived) = 1 OR archived_at IS NULL)
AND id > sqlc.arg(after_id) ORDER BY id LIMIT sqlc.arg(page_limit);

-- name: IsInstanceMainSession :one
SELECT EXISTS(SELECT 1 FROM agent_instances WHERE main_session_id = ? AND archived_at IS NULL);

-- name: StampInstanceMainSession :execrows
UPDATE sessions SET instance_id = sqlc.arg(instance_id)
WHERE sessions.id = sqlc.arg(session_id) AND archived_at IS NULL
AND (parent_session_id IS NULL OR parent_session_id = '')
AND (instance_id IS NULL OR instance_id = sqlc.arg(instance_id))
AND EXISTS (SELECT 1 FROM agent_instances i WHERE i.id = sqlc.arg(instance_id)
AND i.agent_name = sessions.agent_name AND i.profile_id = sessions.profile_id
AND i.workspace_id = sessions.workspace_id AND i.archived_at IS NULL);

-- name: PublishMissingInstanceMain :execrows
UPDATE agent_instances SET main_session_id=sqlc.arg(session_id), updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND main_session_id IS NULL AND archived_at IS NULL;

-- name: ListInstancesForReconciliation :many
SELECT * FROM agent_instances WHERE archived_at IS NULL AND id > sqlc.arg(after_id)
ORDER BY id LIMIT sqlc.arg(page_limit);

-- name: SessionMayHaveInstanceAncestry :one
SELECT COALESCE(instance_id, '') <> '' OR COALESCE(parent_session_id, '') <> ''
FROM sessions WHERE id = sqlc.arg(id);

-- name: GetInstanceCaller :one
WITH RECURSIVE ancestry(id, parent_session_id, instance_id) AS (
SELECT id, parent_session_id, instance_id FROM sessions WHERE id=sqlc.arg(session_id)
UNION
SELECT s.id, s.parent_session_id, s.instance_id FROM sessions s JOIN ancestry a ON s.id=a.parent_session_id
)
SELECT s.id, s.agent_name, s.profile_id, s.permission_policy_json, s.effective_workspace_access, s.lineage_kind, s.session_type,
COALESCE((SELECT i.id FROM ancestry a JOIN agent_instances i ON i.id=a.instance_id LIMIT 1),'') AS originating_instance_id,
COALESCE((SELECT MAX(i.depth) FROM ancestry a JOIN agent_instances i ON i.id=a.instance_id),0) AS instance_depth
FROM sessions s WHERE s.id=sqlc.arg(session_id);

-- name: GetInstanceMainAttempt :one
SELECT id FROM sessions WHERE instance_id = ? AND archived_at IS NULL
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: DetachFailedInstanceAttempts :exec
UPDATE sessions SET instance_id = NULL
WHERE instance_id = sqlc.arg(instance_id) AND archived_at IS NOT NULL
AND EXISTS (SELECT 1 FROM agent_instances i WHERE i.id = sqlc.arg(instance_id)
AND i.state = 'starting' AND i.main_session_id IS NULL);

-- name: IsSupersededInstanceSession :one
SELECT EXISTS (
 SELECT 1 FROM sessions s JOIN agent_instances i ON i.id=s.instance_id
 WHERE s.id=sqlc.arg(session_id) AND i.main_session_id IS NOT NULL
 AND i.main_session_id<>s.id
);

-- name: InstanceNeedsAttention :one
WITH RECURSIVE instance_sessions(id, profile_id) AS (
 SELECT s.id, s.profile_id FROM sessions s JOIN agent_instances i ON s.instance_id = i.id
 WHERE i.id = sqlc.arg(instance_id) AND s.profile_id = i.profile_id
 UNION
 SELECT s.id, s.profile_id FROM sessions s JOIN instance_sessions parent ON s.parent_session_id = parent.id
 WHERE s.profile_id = parent.profile_id
)
SELECT EXISTS (
 SELECT 1 FROM session_pending_interactions p JOIN instance_sessions s ON p.session_id = s.id
 WHERE p.status IN ('pending', 'orphaned')
) OR EXISTS (
 SELECT 1 FROM agent_instance_reports r WHERE r.instance_id = sqlc.arg(instance_id) AND r.state = 'pending' AND r.kind = 'failure'
 AND r.created_at > COALESCE((SELECT last_seen_at FROM agent_instance_seen
 WHERE instance_id = r.instance_id AND operator_id = 'local-operator'), '')
);

-- name: ListInstanceQueuedSessions :many
WITH RECURSIVE releasable_instance(id, profile_id, budget_held) AS (
 SELECT id, profile_id,
   (state = 'paused' AND pause_reason IN ('daily_budget', 'daily_budget_recovery'))
   OR EXISTS (SELECT 1 FROM agent_instance_daily_budget_recovery guard
     WHERE guard.instance_id = agent_instances.id AND guard.latched = 1)
 FROM agent_instances WHERE id = sqlc.arg(instance_id) AND archived_at IS NULL
 AND (state = 'active' OR (state = 'paused' AND pause_reason IN (
   'definition_missing', 'definition_not_always_on', 'daily_budget', 'daily_budget_recovery'
 )))
), instance_sessions(id, profile_id) AS (
 SELECT s.id, s.profile_id FROM releasable_instance i JOIN sessions s ON s.instance_id = i.id AND s.profile_id = i.profile_id
 UNION
 SELECT s.id, s.profile_id FROM sessions s JOIN instance_sessions parent ON s.parent_session_id = parent.id
 WHERE s.profile_id = parent.profile_id
), budget_sessions(id, profile_id) AS (
 SELECT s.id, s.profile_id FROM releasable_instance i JOIN sessions s ON s.instance_id = i.id AND s.profile_id = i.profile_id
 UNION
 SELECT d.id, d.profile_id FROM releasable_instance i
 JOIN deleted_session_budget_lineage d ON d.instance_id = i.id AND d.profile_id = i.profile_id
 WHERE NOT EXISTS (SELECT 1 FROM sessions s WHERE s.id = d.id)
 UNION
 SELECT s.id, s.profile_id FROM budget_sessions parent
 JOIN sessions s ON s.parent_session_id = parent.id AND s.profile_id = parent.profile_id
 WHERE COALESCE(NULLIF(s.instance_id, ''), sqlc.arg(instance_id)) = sqlc.arg(instance_id)
 AND s.lineage_kind NOT IN ('continue', 'fork', 'provenance', 'side')
 UNION
 SELECT d.id, d.profile_id FROM budget_sessions parent
 JOIN deleted_session_budget_lineage d ON d.parent_session_id = parent.id AND d.profile_id = parent.profile_id
 WHERE COALESCE(NULLIF(d.instance_id, ''), sqlc.arg(instance_id)) = sqlc.arg(instance_id)
 AND d.lineage_kind NOT IN ('continue', 'fork', 'provenance', 'side')
 AND NOT EXISTS (SELECT 1 FROM sessions s WHERE s.id = d.id)
), held_budget_sessions(id) AS (
 SELECT id FROM budget_sessions WHERE EXISTS (SELECT 1 FROM releasable_instance WHERE budget_held)
)
SELECT s.id FROM sessions s
WHERE s.id > sqlc.arg(after_id) AND s.archived_at IS NULL
AND (s.id IN (SELECT id FROM instance_sessions) OR s.id IN (SELECT id FROM budget_sessions))
AND (s.state <> 'stopped' OR s.id = (SELECT agent_instances.main_session_id FROM agent_instances WHERE agent_instances.id = sqlc.arg(instance_id)))
AND EXISTS (SELECT 1 FROM session_input_queue q WHERE q.session_id = s.id
 AND q.status = 'queued' AND q.session_generation = s.input_generation
 AND (s.id NOT IN (SELECT id FROM held_budget_sessions) OR (
   q.human_ingress = 1 AND q.app_ingress = 0 AND q.dispatchable = 1
   AND (q.owner_kind IS NULL OR q.owner_kind = 'synthetic')
 ))
 AND NOT EXISTS (
   SELECT 1 FROM sessions root JOIN agent_instances i ON i.id = root.instance_id
   WHERE root.id = COALESCE(NULLIF(s.root_session_id, ''), s.id) AND i.state = 'paused'
   AND (s.instance_id = i.id OR i.archived_at IS NOT NULL
     OR i.pause_reason NOT IN ('definition_missing', 'definition_not_always_on'))
   AND (i.archived_at IS NOT NULL OR i.pause_reason NOT IN ('daily_budget', 'daily_budget_recovery')
     OR s.id NOT IN (SELECT id FROM budget_sessions))
   AND (i.archived_at IS NOT NULL OR s.id NOT IN (SELECT id FROM held_budget_sessions) OR q.origin_class <> 'human')
 ))
ORDER BY s.id LIMIT 100;

-- name: CancelInstanceLineageInputs :exec
UPDATE session_input_queue SET status = 'canceled', canceled_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE status = 'queued' AND session_id IN (
 SELECT target.id FROM sessions target JOIN sessions root ON root.id = COALESCE(NULLIF(target.root_session_id, ''), target.id)
 JOIN agent_instances i ON i.id = root.instance_id
 WHERE i.id = sqlc.arg(instance_id) AND target.profile_id = i.profile_id
);

-- name: CountInstancePausesByReason :many
SELECT pause_reason, COUNT(*) AS count FROM agent_instances
WHERE profile_id = sqlc.arg(profile_id) AND workspace_id = sqlc.arg(workspace_id)
AND state = 'paused' AND archived_at IS NULL GROUP BY pause_reason;

-- name: CountHeldInstanceInputs :one
WITH RECURSIVE scoped_instances AS (
 SELECT i.id, i.profile_id, i.state, i.pause_reason, COALESCE(recovery.latched, 0) AS latched
 FROM agent_instances i
 LEFT JOIN agent_instance_daily_budget_recovery recovery ON recovery.instance_id = i.id
 WHERE i.archived_at IS NULL AND (i.state = 'paused' OR recovery.latched = 1)
 AND ((COALESCE(CAST(sqlc.arg(instance_id) AS TEXT), '') != '' AND i.id = sqlc.arg(instance_id))
   OR (COALESCE(CAST(sqlc.arg(instance_id) AS TEXT), '') = ''
     AND i.profile_id = sqlc.arg(profile_id) AND i.workspace_id = sqlc.arg(workspace_id)))
), paused_instances(id, profile_id, budget_hold) AS (
 SELECT id, profile_id, 0 AS budget_hold FROM scoped_instances
 WHERE state = 'paused' AND pause_reason NOT IN ('daily_budget', 'daily_budget_recovery')
 UNION ALL
 SELECT id, profile_id, 1 AS budget_hold FROM scoped_instances
 WHERE latched = 1 OR (state = 'paused' AND pause_reason IN ('daily_budget', 'daily_budget_recovery'))
), instance_sessions(id, profile_id, instance_id, budget_hold) AS (
 SELECT s.id, s.profile_id, i.id AS instance_id, i.budget_hold
 FROM paused_instances i JOIN sessions s ON s.instance_id = i.id AND s.profile_id = i.profile_id
 UNION
 SELECT d.id, d.profile_id, i.id, i.budget_hold
 FROM paused_instances i JOIN deleted_session_budget_lineage d ON d.instance_id = i.id AND d.profile_id = i.profile_id
 WHERE i.budget_hold = 1 AND NOT EXISTS (SELECT 1 FROM sessions s WHERE s.id = d.id)
 UNION
 SELECT s.id, s.profile_id, parent.instance_id, parent.budget_hold
 FROM instance_sessions parent JOIN sessions s ON s.parent_session_id = parent.id AND s.profile_id = parent.profile_id
 WHERE parent.budget_hold = 0 OR (
   COALESCE(NULLIF(s.instance_id, ''), parent.instance_id) = parent.instance_id
   AND s.lineage_kind NOT IN ('continue', 'fork', 'provenance', 'side')
 )
 UNION
 SELECT d.id, d.profile_id, parent.instance_id, parent.budget_hold
 FROM instance_sessions parent
 JOIN deleted_session_budget_lineage d ON d.parent_session_id = parent.id AND d.profile_id = parent.profile_id
 WHERE parent.budget_hold = 1 AND COALESCE(NULLIF(d.instance_id, ''), parent.instance_id) = parent.instance_id
 AND d.lineage_kind NOT IN ('continue', 'fork', 'provenance', 'side')
 AND NOT EXISTS (SELECT 1 FROM sessions s WHERE s.id = d.id)
), held_rows AS (
 SELECT DISTINCT q.id, q.synthetic_prompt_json FROM instance_sessions owned
 JOIN session_input_queue q ON q.session_id = owned.id
 JOIN sessions current ON current.id = owned.id
 WHERE q.status = 'queued' AND q.session_generation = current.input_generation AND (
   (owned.budget_hold = 1 AND (q.human_ingress = 0 OR q.app_ingress = 1))
   OR (owned.budget_hold = 0 AND q.origin_class <> 'human' AND EXISTS (
     SELECT 1 FROM sessions root JOIN agent_instances i ON i.id = root.instance_id
     WHERE root.id = COALESCE(NULLIF(current.root_session_id, ''), current.id)
     AND i.id = owned.instance_id AND i.state = 'paused'
     AND (current.instance_id = i.id OR i.archived_at IS NOT NULL OR i.pause_reason NOT IN ('definition_missing', 'definition_not_always_on'))
   ))
 )
)
SELECT CAST(COALESCE(SUM(1 + COALESCE(json_array_length(synthetic_prompt_json, '$.coalesced_message_ids'), 0)), 0) AS INTEGER)
FROM held_rows;

-- name: TripInstanceUnattendedPause :execrows
UPDATE agent_instances SET state = 'paused', pause_reason = 'unattended', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND archived_at IS NULL
AND (state = 'active' OR (state = 'paused' AND pause_reason IN ('definition_missing', 'definition_not_always_on')));

-- name: GetSessionInputHold :one
WITH RECURSIVE budget_ancestry(id, instance_id, parent_session_id, lineage_kind, profile_id) AS (
  SELECT seed.id, seed.instance_id, seed.parent_session_id, seed.lineage_kind, seed.profile_id
  FROM sessions seed WHERE seed.id = sqlc.arg(session_id)
  UNION
  SELECT COALESCE(parent.id, deleted.id),
    CASE WHEN parent.id IS NOT NULL THEN parent.instance_id ELSE deleted.instance_id END,
    CASE WHEN parent.id IS NOT NULL THEN parent.parent_session_id ELSE deleted.parent_session_id END,
    COALESCE(parent.lineage_kind, deleted.lineage_kind), COALESCE(parent.profile_id, deleted.profile_id)
  FROM budget_ancestry a
  LEFT JOIN sessions parent ON parent.id = a.parent_session_id
  LEFT JOIN deleted_session_budget_lineage deleted ON parent.id IS NULL AND deleted.id = a.parent_session_id
  WHERE COALESCE(a.instance_id, '') = '' AND COALESCE(parent.profile_id, deleted.profile_id) = a.profile_id
    AND a.lineage_kind NOT IN ('continue', 'fork', 'provenance', 'side')
), budget_owner AS (
  SELECT a.instance_id FROM budget_ancestry a WHERE a.instance_id IS NOT NULL AND a.instance_id != ''
), budget_pause AS (
  SELECT instance.id,
    CAST(CASE WHEN instance.pause_reason = 'daily_budget_recovery'
      THEN 'daily_budget_recovery' ELSE 'daily_budget' END AS TEXT) AS pause_reason
  FROM budget_owner owner JOIN agent_instances instance ON instance.id = owner.instance_id
  LEFT JOIN agent_instance_daily_budget_recovery guard ON guard.instance_id = instance.id
  WHERE instance.archived_at IS NULL AND
    ((instance.state = 'paused' AND instance.pause_reason IN ('daily_budget', 'daily_budget_recovery'))
      OR guard.latched = 1)
), legacy_pause AS (
  SELECT instance.id, CAST(CASE WHEN instance.archived_at IS NOT NULL THEN 'instance_archived'
    ELSE instance.pause_reason END AS TEXT) AS pause_reason
  FROM sessions target
  JOIN sessions root ON root.id = COALESCE(NULLIF(target.root_session_id, ''), target.id)
  JOIN agent_instances instance ON instance.id = root.instance_id
  WHERE target.id = sqlc.arg(session_id) AND instance.state = 'paused'
    AND (instance.archived_at IS NOT NULL OR (
      instance.pause_reason NOT IN ('daily_budget', 'daily_budget_recovery')
      AND (target.instance_id = instance.id
        OR instance.pause_reason NOT IN ('definition_missing', 'definition_not_always_on'))
    ))
), holds AS (
  SELECT b.id, b.pause_reason, 1 AS budget FROM budget_pause b
  UNION ALL
  SELECT l.id, l.pause_reason, 0 AS budget FROM legacy_pause l
)

SELECT h.id, h.pause_reason FROM holds h ORDER BY h.budget DESC LIMIT 1;

-- name: GetHeldSessionInput :one
WITH RECURSIVE budget_ancestry(id, instance_id, parent_session_id, lineage_kind, profile_id) AS (
  SELECT seed.id, seed.instance_id, seed.parent_session_id, seed.lineage_kind, seed.profile_id
  FROM sessions seed WHERE seed.id = sqlc.arg(session_id)
  UNION
  SELECT COALESCE(parent.id, deleted.id),
    CASE WHEN parent.id IS NOT NULL THEN parent.instance_id ELSE deleted.instance_id END,
    CASE WHEN parent.id IS NOT NULL THEN parent.parent_session_id ELSE deleted.parent_session_id END,
    COALESCE(parent.lineage_kind, deleted.lineage_kind), COALESCE(parent.profile_id, deleted.profile_id)
  FROM budget_ancestry a
  LEFT JOIN sessions parent ON parent.id = a.parent_session_id
  LEFT JOIN deleted_session_budget_lineage deleted ON parent.id IS NULL AND deleted.id = a.parent_session_id
  WHERE COALESCE(a.instance_id, '') = '' AND COALESCE(parent.profile_id, deleted.profile_id) = a.profile_id
    AND a.lineage_kind NOT IN ('continue', 'fork', 'provenance', 'side')
), budget_owner AS (
  SELECT a.instance_id FROM budget_ancestry a WHERE a.instance_id IS NOT NULL AND a.instance_id != ''
), budget_pause AS (
  SELECT instance.id,
    CAST(CASE WHEN instance.pause_reason = 'daily_budget_recovery'
      THEN 'daily_budget_recovery' ELSE 'daily_budget' END AS TEXT) AS pause_reason
  FROM budget_owner owner JOIN agent_instances instance ON instance.id = owner.instance_id
  LEFT JOIN agent_instance_daily_budget_recovery guard ON guard.instance_id = instance.id
  WHERE instance.archived_at IS NULL AND
    ((instance.state = 'paused' AND instance.pause_reason IN ('daily_budget', 'daily_budget_recovery'))
      OR guard.latched = 1)
), legacy_pause AS (
  SELECT instance.id, CAST(CASE WHEN instance.archived_at IS NOT NULL THEN 'instance_archived'
    ELSE instance.pause_reason END AS TEXT) AS pause_reason
  FROM sessions target
  JOIN sessions root ON root.id = COALESCE(NULLIF(target.root_session_id, ''), target.id)
  JOIN agent_instances instance ON instance.id = root.instance_id
  WHERE target.id = sqlc.arg(session_id) AND instance.state = 'paused'
    AND (instance.archived_at IS NOT NULL OR (
      instance.pause_reason NOT IN ('daily_budget', 'daily_budget_recovery')
      AND (target.instance_id = instance.id
        OR instance.pause_reason NOT IN ('definition_missing', 'definition_not_always_on'))
    ))
), holds AS (
  SELECT b.id, b.pause_reason, 1 AS budget FROM budget_pause b
  UNION ALL
  SELECT l.id, l.pause_reason, 0 AS budget FROM legacy_pause l
)

SELECT hold.id, hold.pause_reason, CAST(MIN(q.enqueued_at) AS TEXT) AS since
FROM holds hold
JOIN sessions target ON target.id = sqlc.arg(session_id)
JOIN session_input_queue q ON q.session_id = target.id AND q.session_generation = target.input_generation
WHERE q.status = 'queued' AND (
  (hold.budget = 1 AND NOT (q.human_ingress = 1 AND q.app_ingress = 0))
  OR (hold.budget = 0 AND q.origin_class <> 'human')
)
GROUP BY hold.id, hold.pause_reason, hold.budget ORDER BY hold.budget DESC LIMIT 1;

-- name: CountCheckInProposalStates :one
SELECT
 CAST(COALESCE(SUM(CASE WHEN p.actor = 'auto_accept' THEN 1 ELSE 0 END),0) AS INTEGER) AS auto_accepted,
 CAST(COALESCE(SUM(CASE WHEN p.report_id IS NULL THEN 1 ELSE 0 END),0) AS INTEGER) AS waiting
FROM agent_instance_reports r, json_each(r.payload, '$.proposals') proposal
LEFT JOIN agent_instance_proposal_picks p ON p.report_id = r.id AND p.proposal_index = CAST(proposal.key AS INTEGER)
WHERE r.instance_id = sqlc.arg(instance_id) AND r.source_key = sqlc.arg(source_key) AND r.kind = 'proposal';

-- name: CountScopedCheckInCalls :many
SELECT c.access, c.decision, COUNT(*) AS count FROM agent_instance_check_tool_calls c
JOIN agent_instances i ON i.id = c.instance_id
WHERE i.profile_id = sqlc.arg(profile_id) AND i.workspace_id = sqlc.arg(workspace_id) GROUP BY c.access, c.decision;

-- name: CountScopedInstancePicks :many
SELECT p.actor, COUNT(*) AS count FROM agent_instance_proposal_picks p
JOIN agent_instances i ON i.id = p.instance_id
WHERE i.profile_id = sqlc.arg(profile_id) AND i.workspace_id = sqlc.arg(workspace_id) GROUP BY p.actor;

-- name: InstanceHasLiveInteractions :one
WITH RECURSIVE instance_sessions(id, profile_id) AS (
 SELECT s.id, s.profile_id FROM sessions s JOIN agent_instances i ON s.instance_id = i.id
 WHERE i.id = sqlc.arg(instance_id) AND s.profile_id = i.profile_id
 UNION
 SELECT s.id, s.profile_id FROM sessions s JOIN instance_sessions parent ON s.parent_session_id = parent.id
 WHERE s.profile_id = parent.profile_id
)
SELECT EXISTS (
 SELECT 1 FROM session_pending_interactions p JOIN instance_sessions s ON p.session_id = s.id
 JOIN sessions live ON live.id = s.id
 WHERE p.status IN ('pending', 'orphaned') AND live.state <> 'stopped'
);
