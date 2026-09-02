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
retest_status: pass
fix_commits: 45cf683dc135ee25e82658af5981bf54503d40fd; 9ed228b6d8a2cd55687aaf73c2ff0a02be92ac55; f5613df7dfac0725d4499166d6f14dff7e76a6ac; ee410b18fe959e29a60140ed1020448859247070
evidence: .cache/qa-labs/compozy-pr-531-profile-scopes-20260902-170122-478233-lab/qa-artifacts/qa/notes/cli-profile-scope.md; .cache/qa-labs/compozy-pr-531-profile-scopes-20260902-170122-478233-lab/qa-artifacts/qa/notes/api-profile-scope.md; .cache/gate/logs/go-test-1788369547-28708.log
last_report: docs/qa/reports/2026-09-02-pr-531-remediation.md
overlaps: ET-compozy-native-tool-invocation; LP-live-run-survives-extension-disable
---

2026-09-01: `pass` — scoped public validation accepts `ext__qa_lab__capture_candidate` only for its acting workspace/Profile and returns `unknown_action_kind` for a peer workspace. Real extension-provider suites separately prove profile runtime identity, dynamic enable/disable projection, schema validation, and permission gating. An isolated CLI/API/Loop smoke run settled and remained readable after daemon restart; no provider or credential mutation was used.

2026-09-02: reset to `untested` because the final PR tree changes Profile-scoped extension visibility and the prior pass predates that behavior.

2026-09-02: `pass` — fresh CLI/UDS and HTTP reads accepted the lifecycle action in `qa-profile`, rejected the default Profile and peer workspace with `unknown_action_kind`, and preserved the healthy Profile placement across daemon restart.
