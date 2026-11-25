-- SPDX-License-Identifier: FSL-1.1-MIT

-- name: AppendSecurityEvent :one
INSERT INTO security_events (account_id, type, actor, at, details_json)
VALUES (sqlc.narg(account_id), sqlc.arg(event_type), sqlc.arg(actor), sqlc.arg(at),
        sqlc.arg(details_json)::text::jsonb) RETURNING id;

-- name: CompactSecurityEvents :exec
INSERT INTO security_event_retention (cutoff) VALUES (sqlc.arg(cutoff)::timestamptz);
