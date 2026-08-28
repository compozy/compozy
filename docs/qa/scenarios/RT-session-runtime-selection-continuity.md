---
id: RT-session-runtime-selection-continuity
area: RT
title: Preserve the session runtime selection through stop and restart
persona: Théo
journey: J-17
expected: Choosing provider, logical model, Reasoning, Fast, or typed ACP options persists immediately for that session; stop, reopen, refresh, and daemon restart restore the same selected values and revision, and the next prompt uses them without changing earlier turns or the agent default.
entry_points: web session composer; CLI session runtime set|clear; HTTP+UDS session runtime route
qa_status: pass
bug_ids: BUG-20260828-session-runtime-restart-projection
fix_status: fixed-pending-commit
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-alias-prompt.json;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-exact-retry.json;docs/qa/reports/2026-08-13-issue-389-cursor-model.md;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/session-runtime-restart-projection.json
last_report: docs/qa/reports/2026-08-27-acp-runtime-catalog.md
overlaps: RT-072; ET-web-runtime-selector-minimal-slider
---

Added by the 2026-08-04 durable ACP session fix. The selected runtime is session-scoped preference state; the effective runtime remains evidence of what the current or last ACP process actually used.

QA 2026-08-04: Claude Fable 5 with Max reasoning survived manual stop, permalink reopen, browser reload, and daemon restart. HTTP readback and UDS CLI both addressed the same session and the next live prompt used the retained selection.

QA impact 2026-08-13: a Cursor selection now persists only when its model is an exact live ACP value.
Reset for a public selection readback and next-prompt continuity walk.

QA impact 2026-08-27: durable selection now preserves typed ACP options and public logical model IDs.
Reset for store, HTTP/UDS/CLI/native, restart, and next-prompt continuity.

QA 2026-08-28: pass after repair. A real stop and daemon restart initially exposed a projection gap;
after the fix, public list/read state retained Grok 4.6, xhigh, Fast, revision 2, and generation 1.
Canonical store, reconciliation, query, resume, and runtime suites pass with `-race`.
