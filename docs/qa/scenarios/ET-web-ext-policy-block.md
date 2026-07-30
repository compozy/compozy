---
id: ET-web-ext-policy-block
area: ET
title: Respect extension trust policy in Marketplace
persona: Bruno
journey: J-marketplace-acquisition
expected: A blocked daemon trust decision leaves Install focusable but unavailable, explains the real typed warning, links Settings Extensions, and performs no write; an allowed-unverified decision requires explicit confirmation.
entry_points: /marketplace/extension/$entryId; extension Install action; Settings Extensions
qa_status: blocked-verify
bug_ids: BUG-20260714-keyboard-focus-invisible
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/extension-unverified-policy-blocked.png; /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/extension-unverified-policy-live.png; /Users/pedronauck/Dev/compozy/compozy/.tmp/bug-20260714-focus/focused.png;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-018; ET-cli-extension-sideload-policy-block; ET-web-settings-extensions-policy
---

Added by marketplace Task 06. Compare the rendered decision, registry tier, policy, and warning payload with the same daemon response; prove the Web does not infer trust from tier copy and cannot bypass a policy block.

Historical QA note: the default policy block, live policy flip, request consent, zero residue, and keyboard focus passed.
