---
id: ET-extension-agent-observer-resolution
area: ET
title: Observe a resource-defined agent after catalog changes
persona: Ada
journey: J-extension-dev-lifecycle
expected: Global, workspace, and builtin agents resolve through the live resource catalog for provider authentication; changing the catalog invalidates observation authorization state; stopped-session events retain the persisted model, permission mode, and authentication owner without leaking another workspace.
entry_points: extension resource catalog; session prompt; observe HTTP/UDS/native reads; workspace-scoped session events
qa_status: blocked-verify
bug_ids:
fix_status: fixed
retest_status: blocked-verify
fix_commits: 75ce57f2;ed93a4b3
evidence: internal/observe/observer_test.go;internal/observe/observer_session_events_test.go;/Users/pedronauck/dev/qa-labs/compozy-pr-447-runtime-recovery-20260821-020432-748658-lab/qa-artifacts/qa/evidence/catalog-revision-observation.json;/Users/pedronauck/dev/qa-labs/compozy-pr-447-runtime-recovery-20260821-020432-748658-lab/qa-artifacts/qa/evidence/observe-isolation.json
last_report: docs/qa/reports/2026-08-20-pr-447-runtime-recovery.md
overlaps: ET-resource-only-extension-dev; RT-018
---

Activate a resource-defined agent, run a session, change its catalog authorization settings, and generate another observation. Compare live and stopped-session observe reads, then repeat the read from another workspace. The latest catalog revision and persisted session truth must win, and no session or authorization metadata may cross the workspace boundary.

QA 2026-08-20: the live public path passed without a daemon restart: `qa_resource_coder` changed from GPT-5.4/approve-all to GPT-5.6 Sol/deny-all, and session `sess-0818ef9fc543d30c` used the new model. The `pedronauck` workspace observe read remained at zero tokens while `pr447-qa` contained the provider usage. Final status is blocked because no public surface can emit a late stopped-session event or expose the recovered authentication owner and effective permission mode. Human rerun requires an observer integration harness that emits that late event, then compares persisted model, permission, and authorization owner before and after the catalog revision.
