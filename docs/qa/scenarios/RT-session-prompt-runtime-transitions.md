---
id: RT-session-prompt-runtime-transitions
area: RT
title: Transition runtime at a prompt boundary
persona: Théo
journey: J-13
expected: A prompt captures provider, logical model, Reasoning, Fast, and typed ACP options; Compozy applies or compiles them in deterministic order, replaces a launch-bound process when required before dispatch, persists the public logical selection, and restores the prior runtime if the transition fails.
entry_points: web session composer; POST /api/sessions/:sid/prompt over HTTP+UDS; CLI compozy session prompt
qa_status: pass
bug_ids: BUG-20260827-cursor-launch-model-negotiation; BUG-20260827-unbound-session-fast-inheritance
fix_status: fixed-pending-commit
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-alias-prompt.json;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-exact-retry.json;docs/qa/reports/2026-08-13-issue-389-cursor-model.md;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/cursor-grok-runtime-after-fix.json;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-first-prompt-grok45-fast-pass.png
last_report: docs/qa/reports/2026-08-27-acp-runtime-catalog.md
overlaps: RT-018; RT-019; RT-061; RT-062
---

The canonical prompt runtime-transition scenario. It covers the runtime snapshot at dispatch, live reconfiguration versus process replacement, and rollback; RT-018 owns ordinary streaming, RT-019 owns busy-input modes, RT-061 owns reasoning order, and RT-062 owns rejected selections.

QA impact 2026-08-13: Cursor prompt binding rejects a non-advertised model before the at-most-once
dispatch commit and accepts the same identity when retried with the exact live ACP value. Reset for
the direct and Goal command paths.

QA impact 2026-08-27: prompt snapshots now include typed ACP options. Cursor changes resolve private
launch aliases and replace the process atomically; standard ACPs refresh and revalidate after every
option response. Reset for direct, durable-selection, queue, Goal, and replacement paths.

QA 2026-08-28: pass. Focused transition, busy-input, snapshot, Goal, and provider-adapter suites
passed; fresh Grok 4.5/High/Fast and Grok 4.6/xhigh/Fast binds preserved logical public state.
