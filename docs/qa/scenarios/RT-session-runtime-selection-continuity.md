---
id: RT-session-runtime-selection-continuity
area: RT
title: Preserve the session runtime selection through stop and restart
persona: Théo
journey: J-17
expected: Choosing provider, model, reasoning, or speed persists immediately for that session; stop, reopen, refresh, and daemon restart restore the same selected values and revision, and the next prompt uses them without changing earlier turns or the agent default.
entry_points: web session composer; CLI session runtime set|clear; HTTP+UDS session runtime route
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-durable-acp-sessions-20260804-164200-701355-lab/qa-artifacts/qa/evidence/01-selected-claude-fable-max.png;/Users/pedronauck/dev/qa-labs/compozy-durable-acp-sessions-20260804-164200-701355-lab/qa-artifacts/qa/evidence/03-stopped-promptable-selected-runtime.png;/Users/pedronauck/dev/qa-labs/compozy-durable-acp-sessions-20260804-164200-701355-lab/qa-artifacts/qa/evidence/runtime-selection-after-resume.json;/Users/pedronauck/dev/qa-labs/compozy-durable-acp-sessions-20260804-164200-701355-lab/qa-artifacts/qa/evidence/cli-runtime-set.json
last_report: docs/qa/reports/2026-08-04-durable-acp-sessions.md
overlaps: RT-072; ET-web-runtime-selector-minimal-slider
---

Added by the 2026-08-04 durable ACP session fix. The selected runtime is session-scoped preference state; the effective runtime remains evidence of what the current or last ACP process actually used.

QA 2026-08-04: Claude Fable 5 with Max reasoning survived manual stop, permalink reopen, browser reload, and daemon restart. HTTP readback and UDS CLI both addressed the same session and the next live prompt used the retained selection.
