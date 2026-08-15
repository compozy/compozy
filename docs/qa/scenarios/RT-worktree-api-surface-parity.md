---
id: RT-worktree-api-surface-parity
area: RT
title: Read and mutate identical worktree state across agent surfaces
persona: Bruno
journey: J-worktree-management
expected: HTTP, UDS, CLI, and native tools pass name and ID references to one workspace-scoped runtime contract, report not-ready only for a real state refusal, and preserve canonical payloads, error codes, risk metadata, streams, and cache isolation.
entry_points: HTTP/UDS GET|POST /api/workspaces/:workspace_id/worktrees; POST .../adopt; GET .../:worktree_id; GET .../status|exit|stream; POST .../cancel|dismiss|exit/actions|exit/cancel; DELETE .../:worktree_id; GET /api/worktrees/catalog-stream; compozy__worktree_list|inspect|create|remove
qa_status: pass
bug_ids: BUG-20260813-base-ref-accepted-before-validation
fix_status: fixed
retest_status: pass
fix_commits: 0d54b6fe
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/api-status-by-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/api-inspect-removed-by-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/api-inspect-old-id-tombstone.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/api-recreated-ready.json
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
