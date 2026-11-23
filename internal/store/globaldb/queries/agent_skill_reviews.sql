-- name: InsertAgentSkillReview :execrows
INSERT INTO agent_skill_reviews
(id, instance_id, profile_id, workspace_id, agent_name, skill_name, digest, state, draft_path, source_session_id,
created_at, kind, skill_key, base_digest, snapshot_path, proposed_digest, lesson, evidence_session_ids)
SELECT sqlc.arg(id), sqlc.arg(instance_id), sqlc.arg(profile_id), sqlc.arg(workspace_id),
sqlc.arg(agent_name), sqlc.arg(skill_name), sqlc.arg(digest), 'pending', sqlc.arg(draft_path),
sqlc.narg(source_session_id), sqlc.arg(created_at), sqlc.arg(kind), sqlc.arg(skill_key), sqlc.arg(base_digest),
sqlc.arg(snapshot_path), sqlc.arg(proposed_digest), sqlc.arg(lesson),
sqlc.arg(evidence_session_ids)
WHERE (SELECT count(*) FROM agent_skill_reviews WHERE instance_id = sqlc.arg(instance_id) AND state = 'pending') < 5;

-- name: GetAgentSkillReview :one
SELECT * FROM agent_skill_reviews WHERE id = ?;

-- name: GetAgentSkillReviewByIdentity :one
-- Proposal identity (instance, skill, digest, base digest); only pending and rejected reviews participate.
SELECT * FROM agent_skill_reviews
WHERE instance_id = sqlc.arg(instance_id) AND skill_name = sqlc.arg(skill_name) AND digest = sqlc.arg(digest)
AND base_digest = sqlc.arg(base_digest) AND state IN ('pending', 'rejected')
ORDER BY state = 'pending' DESC, created_at DESC, id DESC LIMIT 1;

-- name: ListAgentSkillReviews :many
SELECT * FROM agent_skill_reviews
WHERE profile_id = sqlc.arg(profile_id) AND workspace_id = sqlc.arg(workspace_id)
AND (sqlc.arg(instance_id) = '' OR instance_id = sqlc.arg(instance_id))
AND (sqlc.arg(review_state) = '' OR state = sqlc.arg(review_state))
AND (sqlc.arg(review_kind) = '' OR kind = sqlc.arg(review_kind))
AND (sqlc.arg(review_decided_via) = '' OR decided_via = sqlc.arg(review_decided_via))
AND (created_at > sqlc.arg(after_created_at) OR (created_at = sqlc.arg(after_created_at) AND id > sqlc.arg(after_id)))
ORDER BY created_at, id LIMIT sqlc.arg(page_limit);

-- name: RejectAgentSkillReview :execrows
UPDATE agent_skill_reviews SET state = 'rejected', decided_by = sqlc.arg(decided_by),
decided_at = sqlc.arg(decided_at), decided_via = 'operator'
WHERE id = sqlc.arg(id) AND state = 'pending' AND digest = sqlc.arg(digest);

-- name: ProjectAgentSkillApproval :exec
-- Projection of a committed install-log entry; a review that is no longer pending is left unchanged.
UPDATE agent_skill_reviews SET state = 'approved', decided_by = sqlc.arg(decided_by),
decided_via = sqlc.arg(decided_via), decided_at = sqlc.arg(decided_at),
installed_digest = sqlc.arg(installed_digest), prev_install_marker = sqlc.narg(prev_install_marker)
WHERE id = sqlc.arg(id) AND state = 'pending';

-- name: GetAgentSkillEvidenceSession :one
SELECT profile_id, workspace_id FROM sessions WHERE id = sqlc.arg(id);
