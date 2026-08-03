---
id: ET-web-session-composer-text-entry
area: ET
title: Preserve exact text in the session composer
persona: Bruno
journey: J-17
expected: In a newly created session, sequential keyboard entry preserves every character including spaces, opening and closing the Next prompt runtime selector does not alter the draft, and the same behavior remains correct after a refresh and deep-link return.
entry_points: web agent detail New session; web destination session composer; web session deep link
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-02-composer-space/01-exact-draft.png;docs/qa/evidence/2026-08-02-composer-space/02-after-refresh.png;docs/qa/evidence/2026-08-02-composer-space/03-deep-link.png
last_report: docs/qa/reports/2026-08-02-composer-space.md
overlaps: ET-web-session-prompt-runtime-and-create-navigation;ET-web-runtime-selector-minimal-slider
---

QA impact 2026-08-02: the assistant-ui store notification owner was corrected after sequential keyboard input dropped spaces in React StrictMode. This scenario owns exact composer text entry; runtime snapshot and dispatch behavior remain owned by the overlapping scenarios.

QA verdict 2026-08-02: passed in an isolated real daemon/Web lab. Sequential typing preserved normal spaces, leading/repeated/trailing spaces, and accented text before and after the runtime selector, refresh, and deep-link remount. No browser console warnings or errors were observed.
