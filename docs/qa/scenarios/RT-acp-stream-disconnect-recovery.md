---
id: RT-acp-stream-disconnect-recovery
area: RT
title: Recover after an ACP stream disconnect
persona: Théo
journey: J-dead-session-history-recovery
expected: When an ACP process disconnects after streaming partial output, Compozy preserves delivered chunks and process_exit diagnostics. Status, transcript, history, and recap remain readable without ACP session/load. Explicit attach/resume and a normal prompt are rejected as not attachable before another load attempt; the web UI keeps the original history read-only and can fork a child session in the same workspace with parent_session_id preserved.
entry_points: web session window; POST /api/workspaces/:workspace_id/sessions/:session_id/attach; POST /api/workspaces/:workspace_id/sessions/:session_id/prompt; UDS session resume; UDS session prompt; compozy session prompt -o jsonl; compozy session resume; compozy session recap; compozy session new --parent; compozy__session_prompt; session events; session status
qa_status: pass
bug_ids: 315
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-365-dead-session-attach-20260813-220424-298966-lab/qa-artifacts/qa/evidence/dead-session-web-readonly.png; /Users/pedronauck/dev/qa-labs/compozy-issue-365-dead-session-attach-20260813-220424-298966-lab/qa-artifacts/qa/evidence/dead-session-web-after-reload.png; /Users/pedronauck/dev/qa-labs/compozy-issue-365-dead-session-attach-20260813-220424-298966-lab/qa-artifacts/qa/evidence/dead-attach-http.json; /Users/pedronauck/dev/qa-labs/compozy-issue-365-dead-session-attach-20260813-220424-298966-lab/qa-artifacts/qa/evidence/dead-retry-http.json; /Users/pedronauck/dev/qa-labs/compozy-issue-365-dead-session-attach-20260813-220424-298966-lab/qa-artifacts/qa/evidence/dead-attach-uds.stderr; /Users/pedronauck/dev/qa-labs/compozy-issue-365-dead-session-attach-20260813-220424-298966-lab/qa-artifacts/qa/evidence/dead-retry-uds.stderr; /Users/pedronauck/dev/qa-labs/compozy-issue-365-dead-session-attach-20260813-220424-298966-lab/qa-artifacts/qa/evidence/session-load-count-after-retries.json; /Users/pedronauck/dev/qa-labs/compozy-issue-365-dead-session-attach-20260813-220424-298966-lab/qa-artifacts/qa/evidence/session-list-after-fork.json
last_report: docs/qa/reports/2026-08-13-issue-365-dead-session.md
overlaps: RT-050; RT-051; RT-session-context-rebuild
---

Issue #315 exposed a provider-side disconnect while the final response was still streaming. This
scenario owns the boundary between a completed stream, a disconnected ACP peer, and a later
`process_exit`. The original session becomes durable read-only history: automatic replay is unsafe,
and a new prompt must not keep retrying `session/load` after the peer has died.

The deterministic walk uses the real daemon, SQLite store, HTTP server, UDS server, CLI binary, web
UI, and an ACP subprocess fixture that exits with code 23 after one partial assistant chunk. It then
proves repeated transcript reads remain projection-only, the normal prompt is recoverably refused,
and a forked child preserves the original as durable provenance.

Historical QA retest 2026-08-05: the HTTP stream retained `partial before crash` and then emitted one typed
`process_exit` terminal for exit code 23. At the instant that event became observable, its crash
bundle already contained the exit code and captured stderr. CLI JSONL printed the retained frames
and exited nonzero, while `compozy__session_prompt` returned `tool_backend_failed` with
`backend_dead`. The session stopped within the scenario bound, and a new explicit prompt recovered
the same session without replaying the failed turn. Issue #365 changes that final recovery behavior;
the prior verdict is intentionally reset until the new read-only and fork path is walked.

Issue #365 retest 2026-08-13: the deterministic ACP fixture emitted `partial before crash` then exited
with code 23. Repeated recap reads preserved the same durable history (apart from each read's
generated-at timestamp). HTTP attach and prompt returned 409, and UDS resume and prompt were
refused as not attachable. The ACP diagnostics recorded no later `session/load` calls. The web
session window preserved the original transcript and failure details after a fresh direct link and
reload, then forked a child in the same workspace with the original `parent_session_id`.
