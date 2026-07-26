---
id: ET-web-bundle-preview-activate
area: ET
title: Preview and activate a scoped marketplace bundle
persona: Bruno
journey: J-marketplace-acquisition
expected: Profile, scope, and primary-channel changes request a fresh read-only preview; conflicts remain explicit; Activate projects exactly the previewed resources and updates installed state.
entry_points: /marketplace/bundle/$entryId; bundle Activate action
qa_status: pass
bug_ids: BUG-20260714-keyboard-focus-invisible
fix_status: BUG-20260714-keyboard-focus-invisible fixed
retest_status: Preview remained write-free, activation survived reload in 15 seconds, and keyboard focus passed the shared two-pixel contract
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-under-minute.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/marketplace-bundle-under-minute.png; /Users/pedronauck/Dev/compozy/agh/.tmp/bug-20260714-focus/focused.png
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-web-bundle-activation-detail
---

Added by marketplace Task 06. Cover global and workspace scope, both bind states, a 409 or 422 conflict, loading-disable behavior, cancellation without writes, and fresh-read confirmation after activation.
