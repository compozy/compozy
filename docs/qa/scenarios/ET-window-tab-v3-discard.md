---
id: ET-window-tab-v3-discard
area: ET
title: Upgrade stored layouts from every supported release to tabs
persona: Ada
journey: J-agent-manage-window-tabs
expected: Stored snapshots of version 3, 4 and 5 convert to version 6 on load without losing any window, route, navigation history or desktop; the original bytes are kept under a backup key and the migration status reports `migrated` with counts; a snapshot that cannot convert keeps its bytes, opens a default desktop and reports `failed`; a stored snapshot newer than the daemon is never modified and loads fail with `window_manager_unavailable`; layout documents of version 4, 5 and 6 are accepted by validate/apply/profiles, and other versions are rejected without writing.
entry_points: daemon boot on an older COMPOZY_HOME; compozy layout migration status; compozy layout validate|apply; compozy layout get
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits: a1baedd3a
evidence: /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/test-cases/walk-parity-results.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/test-cases/stored-v3-focus-desktop.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/test-cases/stored-v2.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/logs/daemon-migration-lines.log; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/logs/cli-layout-apply.log; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/qa-audit-report.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-09-01-window-manager-hardening.md
overlaps: ET-window-manager-layout-recovery; MS-layout-profile-cli-roundtrip
---

Derived from J-agent-manage-window-tabs step 4. Covers ADR-012, TechSpec invariant 13, data
integrity on rejection. The earlier greenfield expectation is superseded by SD-013.

qa-impact: 2026-09-01 snapshots and layout documents are version 4. A stored version 3 arrangement is
no longer discarded at boot: the daemon migrates it on load and persists it once under the next
revision (a former focus desktop becomes a regular desktop whose owner stays zoomed on it with its
return anchor; layout history resets). Version 2 and unknown-field documents are still discarded;
`layout apply` still rejects any version other than 4 with the unsupported-version diagnostic and no
converter. Reset to re-walk both the migration and the rejections.

qa-impact: 2026-09-01 the migration now persists the version 4 arrangement once under the next revision (a reconnecting browser kept the pre-migration layout while the revision stayed the same); a former focus desktop survives as a regular desktop hosting its owner as a lifted zoom, and unzoom takes the owner home and drops it. Walked P5a, P5b, P5c.

Policy revalidation (2026-09-04): prior evidence below/above proves the historical implementation, not lossless upgrades. Current code still drops incompatible stored layouts in `internal/daemon/window_manager_repository.go` and resets history in `internal/windowmanager/snapshot_v3.go`; legacy layout documents have no converter. This scenario is `blocked-verify` pending the owning runtime migration fix and a new walk. No new runtime QA was performed during the instruction audit.

qa-impact: 2026-11-13 window-tabs-split S10. The blocked-verify reason above is resolved: incompatible stored layouts are no longer dropped; v3/v4/v5 go through frozen legacy types and a permanent converter, and failures keep the original. Walk: take a COMPOZY_HOME last used by v0.4/v0.5 (version 3/4 layouts) and one by v0.6 (version 5), start this build on each, and confirm `compozy layout migration status -o json` reports `migrated` with `from_version` 3, 4 or 5 and the Web shell shows every former window as a tab. For the failure and retry path use `ET-window-layout-upgrade-failure-retry`; for the v0.6 detail and notice use `ET-window-layout-upgrade-v06`.
