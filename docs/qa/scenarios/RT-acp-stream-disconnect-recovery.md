---
id: RT-acp-stream-disconnect-recovery
area: RT
title: Recover after an ACP stream disconnect
persona: Ada
journey: J-15
expected: When an ACP process disconnects after streaming partial output, Compozy preserves the delivered chunks, emits a terminal error through HTTP and UDS, makes CLI JSONL exit nonzero after printing those frames, returns backend_dead from compozy__session_prompt, records process_exit diagnostics, and restarts the same session only after a new explicit prompt without replaying the failed prompt.
entry_points: POST /api/sessions/:id/prompt; UDS session prompt; compozy session prompt -o jsonl; compozy__session_prompt; session events; session status
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-acp-stream-disconnect-20260805-194318-831236-lab/qa-artifacts/qa/evidence/acp-stream-disconnect/runtime-public-surfaces.log; /Users/pedronauck/dev/qa-labs/compozy-acp-stream-disconnect-20260805-194318-831236-lab/qa-artifacts/qa/journey-log.jsonl
last_report: docs/qa/reports/2026-08-05-acp-stream-disconnect.md
overlaps: RT-050; RT-051; RT-session-context-rebuild
---

Issue #315 exposed a provider-side disconnect while the final response was still streaming. This
scenario owns the boundary between a completed stream, a disconnected ACP peer, and a later
`process_exit`. Recovery is deliberately a new explicit prompt: automatic replay is unsafe because
the failed turn may already have performed external side effects.

The deterministic walk uses the real daemon, SQLite store, HTTP server, UDS server, CLI binary, and
an ACP subprocess fixture that exits with code 23 after one partial assistant chunk. It then proves
the same session accepts a separate continuation prompt and retains both outputs.

QA 2026-08-05: the HTTP prompt stream retained `partial before crash`, emitted a terminal error,
projected `process_exit`, and wrote exit code 23 into the crash bundle. The UDS-backed CLI JSONL
path printed the partial chunk plus the error frame and exited nonzero. The native session prompt tool
returned HTTP 502 with `tool_backend_failed` and `backend_dead`. A different explicit prompt completed
in the same Compozy session, and the transcript retained both outputs without duplication.
