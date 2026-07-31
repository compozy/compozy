---
id: ET-web-session-prompt-runtime-and-create-navigation
area: ET
title: Navigate after creation and select a prompt runtime
persona: Bruno
journey: J-17
expected: From New session, the operator creates a session without a first prompt, the web activates the returned owner workspace before navigation, and the destination composer lets them choose a Next prompt runtime that the first submitted prompt uses exactly once.
entry_points: web Agents Start session; web agent detail New session; web destination session composer
qa_status: pass
bug_ids: BUG-20260730-session-create-window-intent
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-07-30-session-runtime-selector/03-after-create.png;docs/qa/evidence/2026-07-30-session-runtime-selector/04-session-open-after-create.png;docs/qa/evidence/2026-07-30-session-runtime-selector/runtime-selector-proof.md
last_report: docs/qa/reports/2026-07-30-session-runtime-selector.md
overlaps: MS-web-session-simple-advanced-launch; RT-new-session-fast-feedback; RT-063; ET-web-runtime-selector-minimal-slider
---

This end-to-end web walk owns the hand-off between J-17 session creation and the J-13 composer flow. It deliberately does not duplicate the runtime transition assertions in RT-session-prompt-runtime-transitions.
