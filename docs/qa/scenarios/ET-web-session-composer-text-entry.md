---
id: ET-web-session-composer-text-entry
area: ET
title: Preserve exact text in the session composer
persona: Bruno
journey: J-17
expected: In a newly created session, sequential keyboard entry preserves every character including spaces, opening and closing the Next prompt runtime selector does not alter the draft, and the same behavior remains correct after a refresh and deep-link return.
entry_points: web agent detail New session; web destination session composer; web session deep link
qa_status: untested
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits: f54e62b;acbbb25
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/screenshots/07-exact-text-new-session.png;/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/screenshots/08-exact-text-after-refresh.png;/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/screenshots/09-exact-text-deep-link.png;/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/exact-prompt-history.json;/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/screenshots/05-exact-prompt-after-refresh.png
last_report: docs/qa/reports/2026-08-05-session-slash-commands.md
overlaps: ET-web-session-prompt-runtime-and-create-navigation;ET-web-runtime-selector-minimal-slider
---

QA impact 2026-08-02: the assistant-ui store notification owner was corrected after sequential keyboard input dropped spaces in React StrictMode. This scenario owns exact composer text entry; runtime snapshot and dispatch behavior remain owned by the overlapping scenarios.

QA verdict 2026-08-02: passed in an isolated real daemon/Web lab. Sequential typing preserved normal spaces, leading/repeated/trailing spaces, and accented text before and after the runtime selector, refresh, and deep-link remount. No browser console warnings or errors were observed.

QA impact 2026-08-05: reset because the composer input and assistant-ui trigger plumbing changed. The exact-text entry invariant must pass again as the adjacent canary for slash insertion.

QA verdict 2026-08-05: passed in a newly created `slash-operator` session. Sequential keyboard entry preserved leading, repeated, and trailing spaces plus accented text before and after opening the runtime selector, after a full refresh, and from a fresh-browser deep link. No browser errors were observed.

QA impact 2026-08-05 (review remediation): reset because programmatic assistant-ui changes now synchronize through the composer-state owner. Re-walk exact text and refresh persistence on the reviewed head.

QA verdict 2026-08-05 (reviewed head): passed. The composer and persisted transcript retained the Unicode text and repeated spacing in `Revisão 😊 /browser-qa /browser-qa antes   do lançamento`; the same text remained in the session history after a cold Web reload.

QA impact 2026-08-05 (composer redesign): reset — the composer input moved from a textarea to a Lexical contenteditable (@assistant-ui/react-lexical). Re-walk exact character preservation (spaces, Unicode, newlines), draft persistence across remount/refresh/deep link, and runtime-selector interaction with the draft.

QA impact 2026-08-15 (attachments): reset remains `untested` because paste, drop, picker, preview removal, and attachment-aware send now share the composer input path. Re-walk exact text and draft persistence beside attachment interactions.
