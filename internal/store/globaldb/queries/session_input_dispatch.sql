-- name: GetNextDispatchableSessionInput :one
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
    AND a.lineage_kind NOT IN ('continue', 'fork', 'provenance')
), budget_owner AS (
  SELECT a.instance_id FROM budget_ancestry a WHERE a.instance_id IS NOT NULL AND a.instance_id != ''
), prepared_goal AS (
  SELECT CASE WHEN EXISTS (
    SELECT 1 FROM sessions target
    JOIN sessions root ON root.id = COALESCE(NULLIF(target.root_session_id, ''), target.id)
    JOIN agent_instances instance ON instance.id = root.instance_id
    WHERE target.id = sqlc.arg(session_id) AND target.instance_id IS NULL
      AND instance.state = 'paused' AND instance.archived_at IS NULL
      AND instance.pause_reason IN ('definition_missing', 'definition_not_always_on')
  ) THEN NULL ELSE goal_input_batch_max_rowid END AS cutoff FROM session_input_queue
  WHERE session_id = sqlc.arg(session_id) AND owner_kind = 'goal'
    AND status = sqlc.arg(queued_status) AND dispatchable = 0
    AND terminal_at IS NULL AND fence_kind IS NULL
    AND session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
  ORDER BY enqueued_at, id LIMIT 1
)
SELECT queued.* FROM session_input_queue AS queued
WHERE queued.session_id = sqlc.arg(session_id)
  AND (queued.origin_class = 'human' OR NOT EXISTS (
    SELECT 1 FROM sessions AS target
    JOIN sessions AS root ON root.id = COALESCE(NULLIF(target.root_session_id, ''), target.id)
    JOIN agent_instances AS instance ON instance.id = root.instance_id
    WHERE target.id = queued.session_id AND instance.state = 'paused'
      AND (instance.archived_at IS NOT NULL OR (
        instance.pause_reason NOT IN ('daily_budget', 'daily_budget_recovery')
        AND (target.instance_id = instance.id
          OR instance.pause_reason NOT IN ('definition_missing', 'definition_not_always_on'))
      ))
  ))
  -- Priority class is not proof of human ingress: agent self-sends can have
  -- human priority. A daily-budget pause holds them while operator input passes.
  AND ((queued.human_ingress = 1 AND queued.app_ingress = 0) OR NOT EXISTS (
    SELECT 1 FROM budget_owner owner JOIN agent_instances i ON i.id = owner.instance_id
    LEFT JOIN agent_instance_daily_budget_recovery guard ON guard.instance_id = i.id
    WHERE i.archived_at IS NULL
      AND ((i.state = 'paused' AND i.pause_reason IN ('daily_budget', 'daily_budget_recovery')) OR guard.latched = 1)
  ))
  AND queued.status = sqlc.arg(queued_status)
  AND queued.dispatchable = 1
  AND (queued.owner_kind IS NULL OR queued.owner_kind = 'synthetic')
  AND queued.session_generation = (SELECT input_generation FROM sessions WHERE sessions.id = sqlc.arg(session_id))
  AND ((SELECT cutoff FROM prepared_goal) IS NULL
       OR queued.rowid <= (SELECT cutoff FROM prepared_goal)
       OR (queued.delivery = 'interrupt_then_prompt' AND queued.origin_class = 'human')
       OR (queued.origin_class = 'human' AND EXISTS (
         SELECT 1 FROM sessions target JOIN sessions root ON root.id = COALESCE(NULLIF(target.root_session_id, ''), target.id)
         JOIN agent_instances i ON i.id = root.instance_id WHERE target.id = queued.session_id AND i.state = 'paused'
       )))
ORDER BY CASE
           WHEN queued.delivery = 'interrupt_then_prompt' AND queued.origin_class = 'human' THEN 0
           WHEN queued.owner_kind = 'goal' THEN 2
           WHEN (SELECT cutoff FROM prepared_goal) IS NULL
             OR queued.rowid <= (SELECT cutoff FROM prepared_goal) THEN 1
           ELSE 3
         END,
         CASE queued.origin_class WHEN 'human' THEN 0 WHEN 'agent' THEN 1 ELSE 2 END,
         queued.delivery DESC, queued.priority DESC, queued.enqueued_at ASC, queued.id ASC
LIMIT 1;

-- name: PeekNextSessionInput :one
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
    AND a.lineage_kind NOT IN ('continue', 'fork', 'provenance')
), budget_owner AS (
  SELECT a.instance_id FROM budget_ancestry a WHERE a.instance_id IS NOT NULL AND a.instance_id != ''
), prepared_goal AS (
  SELECT CASE WHEN EXISTS (
    SELECT 1 FROM sessions target
    JOIN sessions root ON root.id = COALESCE(NULLIF(target.root_session_id, ''), target.id)
    JOIN agent_instances instance ON instance.id = root.instance_id
    WHERE target.id = sqlc.arg(session_id) AND target.instance_id IS NULL
      AND instance.state = 'paused' AND instance.archived_at IS NULL
      AND instance.pause_reason IN ('definition_missing', 'definition_not_always_on')
  ) THEN NULL ELSE goal_input_batch_max_rowid END AS cutoff FROM session_input_queue
  WHERE session_id = sqlc.arg(session_id) AND owner_kind = 'goal'
    AND status = sqlc.arg(queued_status) AND dispatchable = 0
    AND terminal_at IS NULL AND fence_kind IS NULL
    AND session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
  ORDER BY enqueued_at, id LIMIT 1
)
SELECT queued.* FROM session_input_queue AS queued
WHERE queued.session_id = sqlc.arg(session_id)
  AND (queued.origin_class = 'human' OR NOT EXISTS (
    SELECT 1 FROM sessions AS target
    JOIN sessions AS root ON root.id = COALESCE(NULLIF(target.root_session_id, ''), target.id)
    JOIN agent_instances AS instance ON instance.id = root.instance_id
    WHERE target.id = queued.session_id AND instance.state = 'paused'
      AND (instance.archived_at IS NOT NULL OR (
        instance.pause_reason NOT IN ('daily_budget', 'daily_budget_recovery')
        AND (target.instance_id = instance.id OR queued.owner_kind = 'goal'
          OR instance.pause_reason NOT IN ('definition_missing', 'definition_not_always_on'))
      ))
  ))
  AND ((queued.human_ingress = 1 AND queued.app_ingress = 0) OR NOT EXISTS (
    SELECT 1 FROM budget_owner owner JOIN agent_instances i ON i.id = owner.instance_id
    LEFT JOIN agent_instance_daily_budget_recovery guard ON guard.instance_id = i.id
    WHERE i.archived_at IS NULL
      AND ((i.state = 'paused' AND i.pause_reason IN ('daily_budget', 'daily_budget_recovery')) OR guard.latched = 1)
  ))
  AND queued.status = sqlc.arg(queued_status)
  AND queued.terminal_at IS NULL
  AND queued.session_generation = (SELECT input_generation FROM sessions WHERE sessions.id = sqlc.arg(session_id))
  AND (((queued.owner_kind IS NULL OR queued.owner_kind = 'synthetic') AND queued.dispatchable = 1)
       OR (owner_kind = 'goal' AND queued.dispatchable = 0 AND queued.fence_kind IS NULL))
ORDER BY CASE
           WHEN queued.delivery = 'interrupt_then_prompt' AND queued.origin_class = 'human' THEN 0
           WHEN queued.owner_kind = 'goal' THEN 2
           WHEN (SELECT cutoff FROM prepared_goal) IS NULL
             OR queued.rowid <= (SELECT cutoff FROM prepared_goal) THEN 1
           ELSE 3
         END,
         CASE queued.origin_class WHEN 'human' THEN 0 WHEN 'agent' THEN 1 ELSE 2 END,
         queued.delivery DESC, queued.priority DESC, queued.enqueued_at ASC, queued.id ASC
LIMIT 1;

-- name: ListDispatchableSessionInputs :many
-- Dispatch-ordered candidates for batching behind an already claimed head; same filter as
-- GetNextDispatchableSessionInput, so the Goal cutoff still bounds the batch.
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
    AND a.lineage_kind NOT IN ('continue', 'fork', 'provenance')
), budget_owner AS (
  SELECT a.instance_id FROM budget_ancestry a WHERE a.instance_id IS NOT NULL AND a.instance_id != ''
), prepared_goal AS (
  SELECT CASE WHEN EXISTS (
    SELECT 1 FROM sessions target
    JOIN sessions root ON root.id = COALESCE(NULLIF(target.root_session_id, ''), target.id)
    JOIN agent_instances instance ON instance.id = root.instance_id
    WHERE target.id = sqlc.arg(session_id) AND target.instance_id IS NULL
      AND instance.state = 'paused' AND instance.archived_at IS NULL
      AND instance.pause_reason IN ('definition_missing', 'definition_not_always_on')
  ) THEN NULL ELSE goal_input_batch_max_rowid END AS cutoff FROM session_input_queue
  WHERE session_id = sqlc.arg(session_id) AND owner_kind = 'goal'
    AND status = 'queued' AND dispatchable = 0
    AND terminal_at IS NULL AND fence_kind IS NULL
    AND session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
  ORDER BY enqueued_at, id LIMIT 1
)
SELECT queued.* FROM session_input_queue AS queued
WHERE queued.session_id = sqlc.arg(session_id)
  AND (queued.origin_class = 'human' OR NOT EXISTS (
    SELECT 1 FROM sessions AS target
    JOIN sessions AS root ON root.id = COALESCE(NULLIF(target.root_session_id, ''), target.id)
    JOIN agent_instances AS instance ON instance.id = root.instance_id
    WHERE target.id = queued.session_id AND instance.state = 'paused'
      AND (instance.archived_at IS NOT NULL OR (
        instance.pause_reason NOT IN ('daily_budget', 'daily_budget_recovery')
        AND (target.instance_id = instance.id
          OR instance.pause_reason NOT IN ('definition_missing', 'definition_not_always_on'))
      ))
  ))
  -- Priority class is not proof of human ingress: agent self-sends can have
  -- human priority. A daily-budget pause holds them while operator input passes.
  AND ((queued.human_ingress = 1 AND queued.app_ingress = 0) OR NOT EXISTS (
    SELECT 1 FROM budget_owner owner JOIN agent_instances i ON i.id = owner.instance_id
    LEFT JOIN agent_instance_daily_budget_recovery guard ON guard.instance_id = i.id
    WHERE i.archived_at IS NULL
      AND ((i.state = 'paused' AND i.pause_reason IN ('daily_budget', 'daily_budget_recovery')) OR guard.latched = 1)
  ))
  AND queued.status = 'queued'
  AND queued.dispatchable = 1
  AND (queued.owner_kind IS NULL OR queued.owner_kind = 'synthetic')
  AND queued.session_generation = (SELECT input_generation FROM sessions WHERE sessions.id = sqlc.arg(session_id))
  AND ((SELECT cutoff FROM prepared_goal) IS NULL
       OR queued.rowid <= (SELECT cutoff FROM prepared_goal)
       OR (queued.delivery = 'interrupt_then_prompt' AND queued.origin_class = 'human')
       OR (queued.origin_class = 'human' AND EXISTS (
         SELECT 1 FROM sessions target JOIN sessions root ON root.id = COALESCE(NULLIF(target.root_session_id, ''), target.id)
         JOIN agent_instances i ON i.id = root.instance_id WHERE target.id = queued.session_id AND i.state = 'paused'
       )))
ORDER BY CASE
           WHEN queued.delivery = 'interrupt_then_prompt' AND queued.origin_class = 'human' THEN 0
           WHEN (SELECT cutoff FROM prepared_goal) IS NULL
             OR queued.rowid <= (SELECT cutoff FROM prepared_goal) THEN 1
           ELSE 3
         END,
         CASE queued.origin_class WHEN 'human' THEN 0 WHEN 'agent' THEN 1 ELSE 2 END,
         queued.delivery DESC, queued.priority DESC, queued.enqueued_at ASC, queued.id ASC
LIMIT sqlc.arg(row_limit);
