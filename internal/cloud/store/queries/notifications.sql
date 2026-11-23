-- name: NotifyControlEvent :exec
SELECT pg_notify('cloud_events', jsonb_build_object('schema', current_schema(), 'event', $1::text::jsonb)::text);
