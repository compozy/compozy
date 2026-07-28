-- name: InsertSessionCreationProfile :exec
INSERT INTO session_creation_profiles (profile_ref, profile_json, created_at)
VALUES (sqlc.arg(profile_ref), sqlc.arg(profile_json), sqlc.arg(created_at))
ON CONFLICT(profile_ref) DO NOTHING;

-- name: GetSessionCreationProfileJSON :one
SELECT profile_json FROM session_creation_profiles WHERE profile_ref = sqlc.arg(profile_ref);

-- name: GetSessionCreationIdentity :one
SELECT workspace_id, state, creation_profile_ref, policy_spec_digest, creation_digest
FROM sessions
WHERE id = sqlc.arg(id);

-- name: SetSessionCreationIdentity :execrows
UPDATE sessions
SET creation_profile_ref = sqlc.narg(creation_profile_ref),
    policy_spec_digest = sqlc.narg(policy_spec_digest),
    creation_digest = sqlc.narg(creation_digest)
WHERE id = sqlc.arg(id)
  AND creation_profile_ref IS NULL
  AND policy_spec_digest IS NULL
  AND creation_digest IS NULL;
