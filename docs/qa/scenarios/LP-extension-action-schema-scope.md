---
id: LP-extension-action-schema-scope
area: LP
title: Resolve lifecycle extension actions in the acting Loop scope
persona: Bruno
journey: J-complete-partial-loop
expected: A lifecycle-loaded extension action is available to Loop compile, validate, create, patch, fork, and run only in the workspace and Profile where its extension runtime is enabled; peer scopes receive unknown_action_kind, schemas still reject invalid input, and runtime permission policy remains authoritative.
entry_points: compozy loop validate|create|fork|run; POST /api/workspaces/:workspace_id/loops/:name/validate; extension lifecycle reload; Loop action registry
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: passed
fix_commits: 547ab1a379f7ea26abd05a048d9675705b16522a; 4e9e26ff2f651fb281c106c9d9484d42afc1c309
evidence: go test -race ./internal/extension -run TestExtensionToolProvider(Catalog|Dispatch|Availability) -count=1; go test -race ./internal/daemon -run TestLoopToolSchemaSource|TestDaemonExtensionToolProvider -count=1
last_report: docs/qa/reports/2026-09-01-loop-lifecycle-actions.md
overlaps: ET-compozy-native-tool-invocation; LP-live-run-survives-extension-disable
---

2026-09-01: `pass` — scoped public validation accepts `ext__qa_lab__capture_candidate` only for its acting workspace/Profile and returns `unknown_action_kind` for a peer workspace. Real extension-provider suites separately prove profile runtime identity, dynamic enable/disable projection, schema validation, and permission gating. An isolated CLI/API/Loop smoke run settled and remained readable after daemon restart; no provider or credential mutation was used.
