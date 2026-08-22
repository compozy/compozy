---
id: RT-acp-stream-disconnect-recovery
area: RT
title: Recover after an ACP stream disconnect
persona: Théo
journey: J-dead-session-history-recovery
expected: When an ACP process keeps disconnecting after partial output, Compozy records three recovery attempts, emits one exhausted event and one terminal failure, then preserves a read-only transcript and diagnostics that remain forkable without a fourth attempt.
entry_points: web session window; POST /api/workspaces/:workspace_id/sessions/:session_id/attach; POST /api/workspaces/:workspace_id/sessions/:session_id/prompt; UDS session resume; UDS session prompt; compozy session prompt -o jsonl; compozy session resume; compozy session recap; compozy session new --parent; compozy__session_prompt; session events; session status
qa_status: pass
bug_ids: 315
fix_status: fixed
retest_status: pass
fix_commits: working-tree
evidence: docs/qa/evidence/2026-08-22-acp-runtime-recovery/CH-acp-recovery-exhaustion-history-final.png
last_report: docs/qa/reports/2026-08-22-acp-runtime-recovery.md
overlaps: RT-050; RT-051; RT-session-context-rebuild
---

Issue #315 exposed a provider-side disconnect while the final response was still streaming. This
scenario now owns only the exhausted branch after three automatic recovery attempts. The successful
branch moved to `RT-acp-automatic-recovery`; after exhaustion, the original session becomes durable
read-only history and no fourth attempt may start.

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

Behavior changed 2026-08-22: Compozy now automatically replaces the runtime and replays an
interrupted turn up to three times. The previous passing verdict is stale and intentionally reset;
the next walk must prove the new exhausted event sequence and exactly one terminal marker before
revalidating the read-only history and fork path.

QA retest 2026-08-22: `sess-5ca2e105c94cefa4` recorded three bounded replacement attempts,
one exhausted event, one terminal error, and one provider-failure marker while preserving every
partial chunk. Reload and repeated reads kept the cursor at 17 without a fourth attempt. The web
fork action created `sess-5b317987bad34359` with the original session as its parent, and the
original remained unchanged.
