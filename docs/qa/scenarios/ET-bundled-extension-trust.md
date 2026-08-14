---
id: ET-bundled-extension-trust
area: ET
title: Keep bundled extensions verified
persona: Vera
journey: J-extension-policy-admin
expected: A bundled extension such as spec-cycle reports installed_from=bundled, official registry tier, verified checksum evidence, and a verified trust decision without depending on unverified side-load policy; restart reconciliation preserves enabled state and install time.
entry_points: /marketplace/extension/$entryId?installed_name=spec-cycle; compozy extension provenance spec-cycle -o json; GET /api/extensions/spec-cycle
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/marketplace-extension-evidence.md; /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/spec-cycle-trusted-detail.png
last_report: docs/qa/reports/2026-08-07-critical-runtime-ui-fixes.md
overlaps: ET-022; ET-052; ET-web-extension-detail
---

Bundled provenance is first-party runtime evidence, separate from the policy that governs external side-loads.
