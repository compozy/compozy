---
id: ET-window-tab-v3-discard
area: ET
title: Preserve user layouts across version upgrades
persona: Ada
journey: J-agent-manage-window-tabs
expected: Stored layouts from supported releases upgrade losslessly, including navigation and reopen history; an unsupported document is rejected without deleting or overwriting its source. Public layout interfaces follow SD-013 compatibility windows.
entry_points: compozy layout export|validate|apply; compozy layout-profile get|put; HTTP and UDS layout endpoints; skills/compozy/references/window-management.md
qa_status: blocked-verify
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
