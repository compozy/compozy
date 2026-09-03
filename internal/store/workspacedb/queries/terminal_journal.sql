-- name: InsertTerminalCommand :exec
INSERT INTO terminal_commands (
  id, terminal_id, profile_id, actor_kind, actor_id, session_id, run_id,
  command, argv_digest, cwd, started_at, duration_ms, exit_code, exit_signal,
  exit_cause, detected_by, approval, output_bytes, truncated, recording_id
) VALUES (
  sqlc.arg(id), sqlc.narg(terminal_id), sqlc.arg(profile_id), sqlc.arg(actor_kind),
  sqlc.arg(actor_id), sqlc.narg(session_id), sqlc.narg(run_id), sqlc.arg(command),
  sqlc.narg(argv_digest), sqlc.arg(cwd), sqlc.arg(started_at), sqlc.narg(duration_ms),
  sqlc.narg(exit_code), sqlc.narg(exit_signal), sqlc.arg(exit_cause),
  sqlc.arg(detected_by), sqlc.arg(approval), sqlc.arg(output_bytes),
  sqlc.arg(truncated), sqlc.narg(recording_id)
);

-- name: UpdateTerminalCommandRecording :exec
UPDATE terminal_commands
SET recording_id = sqlc.arg(recording_id)
WHERE terminal_id = sqlc.arg(terminal_id)
  AND profile_id = sqlc.arg(profile_id)
  AND started_at >= sqlc.arg(recording_started_at)
  AND (sqlc.narg(recording_stopped_at) IS NULL OR started_at <= sqlc.narg(recording_stopped_at))
  AND recording_id IS NULL;

-- name: TerminalCommandIDExists :one
SELECT EXISTS(
  SELECT 1
  FROM terminal_commands
  WHERE id = sqlc.arg(id)
);

-- name: TerminalRecordingIDExists :one
SELECT EXISTS(
  SELECT 1
  FROM terminal_recordings
  WHERE id = sqlc.arg(id)
);

-- name: FindTerminalRecordingForCommand :one
SELECT id
FROM terminal_recordings
WHERE terminal_id = sqlc.arg(terminal_id)
  AND profile_id = sqlc.arg(profile_id)
  AND sqlc.arg(started_at) >= started_at
  AND (stopped_at IS NULL OR sqlc.arg(started_at) <= stopped_at)
ORDER BY started_at ASC, id ASC
LIMIT 1;

-- name: InsertTerminalArtifact :exec
INSERT INTO terminal_artifacts (
  id, terminal_id, command_id, profile_id, digest, path, bytes, expires_at
) VALUES (
  sqlc.arg(id), sqlc.narg(terminal_id), sqlc.arg(command_id), sqlc.arg(profile_id),
  sqlc.arg(digest), sqlc.arg(path), sqlc.arg(bytes), sqlc.arg(expires_at)
);

-- name: GetTerminalArtifact :one
SELECT id, terminal_id, command_id, profile_id, digest, path, bytes, expires_at
FROM terminal_artifacts
WHERE id = sqlc.arg(id);

-- name: InsertTerminalRecording :exec
INSERT INTO terminal_recordings (
  id, terminal_id, profile_id, digest, path, started_at, stopped_at, bytes, expires_at
) VALUES (
  sqlc.arg(id), sqlc.arg(terminal_id), sqlc.arg(profile_id), sqlc.arg(digest),
  sqlc.arg(path), sqlc.arg(started_at), sqlc.narg(stopped_at), sqlc.arg(bytes),
  sqlc.arg(expires_at)
);

-- name: GetTerminalRecording :one
SELECT id, terminal_id, profile_id, digest, path, started_at, stopped_at, bytes, expires_at
FROM terminal_recordings
WHERE id = sqlc.arg(id);

-- name: ListExpiredTerminalArtifacts :many
SELECT id, path FROM terminal_artifacts
WHERE expires_at <= sqlc.arg(expires_at)
ORDER BY id;

-- name: CountOtherTerminalArtifactPathRefs :one
SELECT COUNT(*) FROM terminal_artifacts
WHERE path = sqlc.arg(path) AND id <> sqlc.arg(id);

-- name: DeleteTerminalArtifact :exec
DELETE FROM terminal_artifacts WHERE id = sqlc.arg(id);

-- name: ListExpiredTerminalRecordings :many
SELECT id, path FROM terminal_recordings
WHERE expires_at <= sqlc.arg(expires_at)
ORDER BY id;

-- name: CountOtherTerminalRecordingPathRefs :one
SELECT COUNT(*) FROM terminal_recordings
WHERE path = sqlc.arg(path) AND id <> sqlc.arg(id);

-- name: DeleteTerminalRecording :exec
DELETE FROM terminal_recordings WHERE id = sqlc.arg(id);

-- name: ClearExpiredTerminalRecordingLinks :exec
UPDATE terminal_commands
SET recording_id = NULL
WHERE recording_id IN (
  SELECT id FROM terminal_recordings WHERE expires_at <= sqlc.arg(expires_at)
);
