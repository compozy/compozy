---
id: RT-acp-stream-disconnect-recovery
area: RT
title: Recover after an ACP stream disconnect
persona: Ada
journey: J-15
expected: When an ACP process disconnects after streaming partial output, Compozy preserves the delivered chunks, emits a terminal error through HTTP and UDS, makes CLI JSONL exit nonzero after printing those frames, returns backend_dead from compozy__session_prompt, records process_exit diagnostics, and restarts the same session only after a new explicit prompt without replaying the failed prompt.
entry_points: POST /api/sessions/:id/prompt; UDS session prompt; compozy session prompt -o jsonl; compozy__session_prompt; session events; session status
qa_status: pass
bug_ids: 315
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-pr319-acp-stream-remediation-20260805-220911-045783-lab/qa-artifacts/qa/evidence/acp-stream-remediation/runtime-public-surfaces.log; /Users/pedronauck/dev/qa-labs/compozy-pr319-acp-stream-remediation-20260805-220911-045783-lab/qa-artifacts/qa/journey-log.jsonl
last_report: docs/qa/reports/2026-08-05-pr319-acp-stream-remediation.md
overlaps: RT-050; RT-051; RT-session-context-rebuild
---

Issue #315 exposed a provider-side disconnect while the final response was still streaming. This
scenario owns the boundary between a completed stream, a disconnected ACP peer, and a later
`process_exit`. Recovery is deliberately a new explicit prompt: automatic replay is unsafe because
the failed turn may already have performed external side effects.

The deterministic walk uses the real daemon, SQLite store, HTTP server, UDS server, CLI binary, and
an ACP subprocess fixture that exits with code 23 after one partial assistant chunk. It then proves
the same session accepts a separate continuation prompt and retains both outputs.

QA retest 2026-08-05: the HTTP stream retained `partial before crash` and then emitted one typed
`process_exit` terminal for exit code 23. At the instant that event became observable, its crash
bundle already contained the exit code and captured stderr. CLI JSONL printed the retained frames
and exited nonzero, while `compozy__session_prompt` returned `tool_backend_failed` with
`backend_dead`. The session stopped within the scenario bound, and a new explicit prompt recovered
the same session without replaying the failed turn.
