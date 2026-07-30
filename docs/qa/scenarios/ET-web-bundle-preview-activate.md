---
id: ET-web-bundle-preview-activate
area: ET
title: Preview and activate a scoped marketplace bundle
persona: Bruno
journey: J-marketplace-acquisition
expected: Profile, scope, and primary-channel changes request a fresh read-only preview; conflicts remain explicit; Activate projects exactly the previewed resources and updates installed state.
entry_points: /marketplace/bundle/$entryId; bundle Activate action
qa_status: blocked-verify
bug_ids: BUG-20260714-keyboard-focus-invisible
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-under-minute.json; /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/marketplace-bundle-under-minute.png; /Users/pedronauck/Dev/compozy/compozy/.tmp/bug-20260714-focus/focused.png;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-web-bundle-activation-detail
---

Added by marketplace Task 06. Cover global and workspace scope, both bind states, a 409 or 422 conflict, loading-disable behavior, cancellation without writes, and fresh-read confirmation after activation.

Historical QA note: the preview remained write-free, activation survived reload in 15 seconds, and keyboard focus passed the shared two-pixel contract.

QA impact 2026-07-26: accepted activation and trust-confirmation operations reject dialog dismissal
until settlement and publish lifecycle-independent feedback. Historical evidence is retained; status
reset to untested and no QA replay ran.

QA impact 2026-07-28: changing workspace while the activation dialog remains mounted now creates a
new workflow and preview for the selected scope. Status remains untested; no QA replay ran.
