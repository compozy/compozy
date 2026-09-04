---
id: ET-profile-workspace-tool-isolation
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

QA impact 2026-09-04: added for the Profile selector regression on operator tool and toolset
projections. The targeted isolated walk must prove owning Profile/Workspace visibility, default and
peer isolation, public CLI/API parity, approval-token Profile/Workspace binding, and clean teardown.
