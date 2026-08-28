---
id: RT-cursor-logical-runtime-options
area: RT
title: Apply Cursor logical model options through private launch bindings
persona: Théo
journey: J-17
expected: Grok 4.5 and 4.6 Reasoning/Fast combinations and Opus 5 Thinking variants resolve to private Cursor aliases before launch; live available models remain admissible outside the curated browsing view; changing a launch-bound value atomically replaces the process without renegotiating against shared ACP configuration state, and public state keeps only the logical model and typed options.
entry_points: web session composer; CLI session prompt|runtime set; HTTP/UDS prompt runtime; session status and events
qa_status: pass
bug_ids: BUG-20260827-cursor-launch-model-negotiation; BUG-20260827-live-uncurated-model-admission; BUG-20260827-unbound-session-fast-inheritance
fix_status: fixed-pending-commit
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/cursor-grok-runtime-after-fix.json;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-first-prompt-grok45-fast-pass.png
last_report: docs/qa/reports/2026-08-27-acp-runtime-catalog.md
overlaps: MS-cursor-account-model-discovery; RT-session-prompt-runtime-transitions; RT-session-runtime-selection-continuity
---

Added for the ACP runtime catalog rebuild. Cover Grok 4.5 low/medium/high with Fast on/off, Grok 4.6
through xhigh with Fast, Opus 5 Thinking variants, and rejection of an invalid combination.

QA 2026-08-27: Grok 4.6 `xhigh` + Fast and Grok 4.5 `high` + Fast first-prompt canaries pass.
Launch-bound selection is validated before start and no longer compared with Cursor's shared ACP
configuration state.

QA 2026-08-28: pass. Canonical catalog and runtime suites cover every advertised Grok 4.5/4.6
reasoning/Fast combination, Opus 5 Thinking variants, invalid combinations, replacement, and public
logical projection; the two real Cursor canaries passed the provider boundary.
