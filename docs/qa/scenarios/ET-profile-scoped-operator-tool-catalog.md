---
id: ET-profile-scoped-operator-tool-catalog
area: ET
title: Project operator tools in the selected Profile and Workspace
persona: Bruno
journey: J-run-extension-commands
expected: Operator tool list, search, info, approval, invoke, and toolset reads resolve the selected Profile; an extension tool appears only in its owning Profile and Workspace, while default and peer scopes remain isolated and denied calls preserve structured permission errors.
entry_points: compozy --profile <name> tool list|search|info|approve|invoke; GET|POST /api/tools; GET /api/toolsets
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-compozy-native-tool-invocation; LP-extension-action-schema-scope
---

QA impact 2026-09-04: added for the Profile selector regression on operator tool and toolset projections. A focused isolated runtime walk must prove the owning Profile/Workspace, default and peer isolation, and structured permission denial before this scenario can pass.
