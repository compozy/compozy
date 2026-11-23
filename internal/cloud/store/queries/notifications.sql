-- name: AdvanceSecurityEpoch :one
UPDATE accounts SET security_epoch = security_epoch + 1 WHERE id = $1 RETURNING security_epoch;

-- name: NotifyControlEvent :exec
SELECT pg_notify('cloud_events', jsonb_build_object('schema', current_schema(), 'event', $1::text::jsonb)::text);
