---
id: ET-web-session-prompt-runtime-and-create-navigation
area: ET
title: Navigate after creation and select a prompt runtime
persona: Bruno
journey: J-17
expected: From New session, the operator creates a session without a first prompt, the web activates the returned owner workspace before navigation, and the destination composer persists a selected runtime that the first and later prompts inherit until it is changed or cleared.
entry_points: web Agents Start session; web agent detail New session; web destination session composer
qa_status: pass
bug_ids: BUG-20260730-session-create-window-intent; BUG-20260827-session-create-first-message-regression; BUG-20260827-unbound-session-fast-inheritance
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-08-04-durable-acp-sessions.md;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-first-prompt-grok45-fast-pass.png
last_report: docs/qa/reports/2026-08-27-acp-runtime-catalog.md
overlaps: MS-web-session-simple-advanced-launch; RT-new-session-fast-feedback; RT-063; ET-web-runtime-selector-minimal-slider
---

This end-to-end web walk owns the hand-off between J-17 session creation and the J-13 composer flow. It deliberately does not duplicate the runtime transition assertions in RT-session-prompt-runtime-transitions.

QA pass 2026-08-04: the selected Claude Fable 5 / Max runtime remained visible after stop and
daemon restart, then the same durable session resumed with its prior transcript. The dedicated
runtime-continuity scenario owns the selected/effective/revision details.

QA impact 2026-08-27: targeted QA found that Start session had regained a First message field.
The field is removed and prompt staging is limited to the already-sent composer fallback; reset to
untested for a complete launch-to-composer replay.

QA 2026-08-27: a fresh Start session navigated without a launch prompt, the destination composer
showed Grok 4.5 High/Fast before bind, and its separate first prompt completed once. CLI readback
kept the same effective runtime after bind.
