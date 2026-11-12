---
id: ET-bundled-extension-trust
area: ET
title: Keep bundled extensions verified
persona: Vera
journey: J-extension-policy-admin
expected: A bundled extension such as spec-cycle reports installed_from=bundled, official registry tier, verified checksum evidence, and a verified trust decision without depending on unverified side-load policy; restart reconciliation preserves enabled state and install time.
entry_points: /marketplace/extension/$entryId?installed_name=spec-cycle; compozy extension provenance spec-cycle -o json; GET /api/extensions/spec-cycle
qa_status: untested
bug_ids:
fix_status: fixed
retest_status: untested
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/marketplace-extension-evidence.md; /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/spec-cycle-trusted-detail.png
last_report: docs/qa/reports/2026-08-07-critical-runtime-ui-fixes.md
overlaps: ET-022; ET-052; ET-web-extension-detail
---

Bundled provenance is first-party runtime evidence, separate from the policy that governs external side-loads.

QA 2026-09-10: The existing daemon boot integration verifies that one corrupt installed manifest remains diagnosable while healthy extensions keep working; declared-profile reconciliation no longer aborts startup. See [release integration recovery](../reports/2026-09-10-release-integration-repair.md).

Upgrade recovery coverage (v0.6.1): seed a registered bundled `dev-cycle` with disabled profile
enablement and scoped installations, then boot the candidate with its `spec-cycle` successor.
Verify the old registration, enablement, scopes, inputs, credentials and package files remain
unchanged. The successor uses its default enablement and does not inherit predecessor scopes.
Doctor must retain actionable diagnostics naming the actual predecessor attachment; use its
scope-specific removal command only after preserving inputs and credentials. A user-owned package
with the same name must remain untouched.

The owning registry and daemon boot integration suites cover retention in CI. The earlier
[targeted upgrade and recovery evidence](../reports/2026-11-01-superseded-extension-recovery.md)
covered the now-expired transfer window; it does not verify this v0.6.1 scenario. Run v0.6.0 before
upgrading if compatible state still needs automatic transfer. No new local walk is authorized.
