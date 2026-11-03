-- name: InsertAgentSkillReview :execrows
INSERT INTO agent_skill_reviews
(id, instance_id, profile_id, workspace_id, agent_name, skill_name, digest, state, draft_path, source_session_id, created_at)
SELECT sqlc.arg(id), sqlc.arg(instance_id), sqlc.arg(profile_id), sqlc.arg(workspace_id),
sqlc.arg(agent_name), sqlc.arg(skill_name), sqlc.arg(digest), 'pending', sqlc.arg(draft_path),
sqlc.narg(source_session_id), sqlc.arg(created_at)
WHERE (SELECT count(*) FROM agent_skill_reviews WHERE instance_id = sqlc.arg(instance_id) AND state = 'pending') < 5
AND NOT EXISTS (SELECT 1 FROM agent_skill_reviews WHERE instance_id = sqlc.arg(instance_id)
AND skill_name = sqlc.arg(skill_name) AND digest = sqlc.arg(digest));

-- name: GetAgentSkillReview :one
SELECT * FROM agent_skill_reviews WHERE id = ?;

-- name: GetAgentSkillReviewByDigest :one
SELECT * FROM agent_skill_reviews WHERE instance_id = ? AND skill_name = ? AND digest = ?
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: ListAgentSkillReviews :many
SELECT * FROM agent_skill_reviews
WHERE profile_id = sqlc.arg(profile_id) AND workspace_id = sqlc.arg(workspace_id)
AND (sqlc.arg(instance_id) = '' OR instance_id = sqlc.arg(instance_id))
AND (sqlc.arg(review_state) = '' OR state = sqlc.arg(review_state))
AND (created_at > sqlc.arg(after_created_at) OR (created_at = sqlc.arg(after_created_at) AND id > sqlc.arg(after_id)))
ORDER BY created_at, id LIMIT sqlc.arg(page_limit);

-- name: DecideAgentSkillReview :execrows
UPDATE agent_skill_reviews SET state = sqlc.arg(state), decided_by = sqlc.arg(decided_by), decided_at = sqlc.arg(decided_at)
WHERE id = sqlc.arg(id) AND state = 'pending' AND digest = sqlc.arg(digest)
AND (sqlc.arg(state) = 'approved' OR sqlc.arg(state) = 'rejected');
