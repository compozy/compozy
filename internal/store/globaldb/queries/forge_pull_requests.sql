-- name: UpsertForgePullRequest :exec
INSERT INTO forge_pull_requests (
 workspace_id, repo_key, branch, provider, pr_number, pr_url, pr_state, merged, draft, head_sha, base_ref, checks_state, checks_passed, checks_failed, checks_pending, checks_failed_names, review_decision, mergeable, pr_updated_at, observed_at, lookup_state, lookup_cause, attempt_at, failure_count, retry_after
) VALUES (
 sqlc.arg(workspace_id), sqlc.arg(repo_key), sqlc.arg(branch), sqlc.arg(provider), sqlc.narg(pr_number), sqlc.arg(pr_url), sqlc.narg(pr_state), sqlc.narg(merged), sqlc.narg(draft), sqlc.arg(head_sha), sqlc.arg(base_ref), sqlc.arg(checks_state), sqlc.arg(checks_passed), sqlc.arg(checks_failed), sqlc.arg(checks_pending), sqlc.arg(checks_failed_names), sqlc.arg(review_decision), sqlc.arg(mergeable), sqlc.narg(pr_updated_at), sqlc.narg(observed_at), sqlc.arg(lookup_state), sqlc.arg(lookup_cause), sqlc.narg(attempt_at), sqlc.arg(failure_count), sqlc.narg(retry_after)
) ON CONFLICT(workspace_id, repo_key, branch) DO UPDATE SET
 provider = excluded.provider,
 pr_number = excluded.pr_number,
 pr_url = excluded.pr_url,
 pr_state = excluded.pr_state,
 merged = excluded.merged,
 draft = excluded.draft,
 head_sha = excluded.head_sha,
 base_ref = excluded.base_ref,
 checks_state = excluded.checks_state,
 checks_passed = excluded.checks_passed,
 checks_failed = excluded.checks_failed,
 checks_pending = excluded.checks_pending,
 checks_failed_names = excluded.checks_failed_names,
 review_decision = excluded.review_decision,
 mergeable = excluded.mergeable,
 pr_updated_at = excluded.pr_updated_at,
 observed_at = excluded.observed_at,
 lookup_state = excluded.lookup_state,
 lookup_cause = excluded.lookup_cause,
 attempt_at = excluded.attempt_at,
 failure_count = excluded.failure_count,
 retry_after = excluded.retry_after;

-- name: ReadForgePullRequests :many
SELECT f.* FROM forge_pull_requests f JOIN json_each(sqlc.arg(keys)) k
 ON f.workspace_id = json_extract(k.value, '$.workspace_id')
 AND f.repo_key = json_extract(k.value, '$.repo_key')
 AND f.branch = json_extract(k.value, '$.branch');
