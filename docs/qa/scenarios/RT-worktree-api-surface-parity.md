---
id: RT-worktree-api-surface-parity
area: RT
title: Read and mutate identical worktree state across agent surfaces
persona: Bruno
journey: J-worktree-management
expected: HTTP, UDS, CLI, native tools, and both worktree streams pass name and ID references to one workspace-scoped runtime contract, report not-ready only for a real state refusal, and preserve canonical payloads, deterministic error codes, risk and approval metadata, ordered replay, redacted events, repository capability diagnostics, and cache isolation.
entry_points: HTTP/UDS GET|POST /api/workspaces/:workspace_id/worktrees; POST .../adopt; POST .../discovered/remove; GET .../:worktree_id; GET .../status|exit|stream; POST .../cancel|dismiss|exit/actions|exit/cancel; DELETE .../:worktree_id; GET /api/worktrees/catalog-stream; compozy__worktree_list|inspect|create|remove
qa_status: pass
bug_ids: BUG-20260813-base-ref-accepted-before-validation
fix_status: fixed
retest_status: pass
fix_commits: 0d54b6fe
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/http-worktree-inspect.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/uds-worktree-inspect.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/native-worktree-inspect.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/base-ref-refusal-fixed.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/api-status-by-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/api-inspect-removed-by-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/api-inspect-old-id-tombstone.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/api-recreated-ready.json
last_report: docs/qa/reports/2026-08-14-worktree-lifecycle-fixes.md
overlaps: RT-worktree-cli-lifecycle
---

QA impact: Task 02 adds the full agent-operable worktree surface. The Phase C walk must compare
payloads and refusal bodies across transports, verify foreign-workspace calls disclose no target
data, assert native risk/approval/capability metadata, and reconnect through `after_sequence` and
`Last-Event-ID` without gaps, duplicate events, unredacted diagnostics, or cache crossover.

QA impact: Task 05 adds exit planning, actions, cancellation, and progress events. The Phase C walk
must compare the complete exit payload and deterministic failures across HTTP, UDS, and CLI.

2026-08-13 fix replay: an invalid base ref now returns HTTP `409` with the canonical
`base_ref_not_found` code before pending persistence; a refreshed listing contains no rejected row.
The complete cross-transport, stream replay, isolation, and exit matrix remains in this charter.

2026-08-14 lifecycle fix flag: repeat ref-taking reads and mutations by name across HTTP, UDS, CLI,
and the supported native tools; confirm the same canonical row and refusal code at every boundary.

2026-08-14 targeted walk: direct HTTP status returned canonical ID `wt_d6b01c5928bcc5fa` for the
name reference; remove and dismiss reached the same row; name lookup stopped resolving the dismissed
tombstone while ID lookup preserved it; recreation with the retained branch produced ready row
`wt_f593dc7d824caa9d`. UDS was exercised by the CLI journey, and the canonical transport suites own
the exhaustive name/ID and native-tool parity matrix.


2026-09-16 bulk-removal impact: both menubar and overview expose bounded selection and individual
missing-record resolution. Verify mixed and missing-only selections, Escape/Shift/select-all,
active/foreign/archived/discovered exclusions, immutable targets during live updates, per-item
partial results and failed-only retry. Metadata dismissal preserves replacements and history;
repeated dismissal and refresh/restart stay clean. No force or session stop is implicit.
Current run: `docs/qa/reports/2026-09-16-worktree-bulk-delete.md` (targeted rendered and API evidence recorded; final CI pending).

2026-10-19 #813 impact: HTTP/UDS `DELETE …/worktrees/{worktree_id}` and `POST …/dismiss` accept
`delete_branch` (dismiss also `force`) and answer `200 {"branch": {...}}` only when it is set;
requests without it keep `204`. `compozy__worktree_remove` accepts `delete_branch` and returns the
same outcome as `branch`. Parity is covered by the handler, transport-parity and native-tool suites.

2026-10-20 discovered-removal impact: `POST …/worktrees/discovered/remove` `{path, force?}` on HTTP
and UDS, and `compozy__worktree_remove` `path`. Isolated-lab walk: HTTP detached HEAD with a unique
commit → 409 `worktree_unpushed_requires_force` (1 commit), forced → 204; UDS clean live → 204;
registered path → 409 `worktree_registered` with checkout intact; main checkout → 409
`adoption_main_checkout`; unlisted → 404; blank → 400. Byte parity is owned by the transport-parity
suite; the native-tool suite owns the `path` input.

Detached-worktree regression: request status details with explicit forge refresh while the stored
branch is empty. Return local status with no forge facts, without a Git/forge lookup or a spurious
cancellation. Repeat with the forge display service absent. Branched worktrees retain the existing
refresh and provider-unavailable behavior. Automated coverage: `TestServiceStatus`.

## Native error parity — 2026-10-28

Native refusal parity (#1012): duplicate creation returns `worktree_name_taken`; removal while session-bound returns `worktree_session_active` with a readable explanation. A real workspace prune failure retains `tool_backend_failed` / `backend_unhealthy` and a sanitized cause. Verify that refusals do not replace or remove the checkout.

The [targeted report](../reports/2026-10-28-native-tool-errors.md) records the real isolated-daemon re-walk, automated evidence, and limits. Broader scenario status is unchanged.

2026-11-01 responsiveness audit impact: compare HTTP and UDS aggregate worktree reads with
`all_workspaces=true`, a selected profile, and explicit `workspace_ids`. Only selected registered
workspaces may reach metadata hydration; an empty authorized selection returns no rows, and a
workspace-catalog failure stops before hydration. Verify unchanged owner labels, deterministic
ordering, default archived-workspace exclusion and explicitly selected history. The API admission
suite and real-SQLite worktree suite own automated evidence; this revision's transport/browser
scenario re-walk remains pending. Evidence and limits: [API and Tasks audit](../reports/2026-11-01-performance/api-tasks.md)
and [worktree audit](../reports/2026-11-01-performance/worktree-workspace.md).
