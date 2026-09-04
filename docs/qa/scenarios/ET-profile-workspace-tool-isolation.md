---
id: ET-profile-workspace-tool-isolation
area: ET
title: Project operator tools in the selected Profile and Workspace
persona: Bruno
journey: J-run-extension-commands
expected: Operator tool list, search, info, approval, invoke, and toolset reads resolve the selected Profile; an extension tool appears only in its owning Profile and Workspace, while default and peer scopes remain isolated and denied calls preserve structured permission errors.
entry_points: compozy --profile <name> tool list|search|info|approve|invoke; GET|POST /api/tools; GET /api/toolsets
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits: 40587df371436a9c239dd9f6d4b76b4419af6690
evidence: docs/qa/reports/2026-09-04-profile-workspace-tool-isolation.md
last_report: docs/qa/reports/2026-09-04-profile-workspace-tool-isolation.md
overlaps: ET-compozy-native-tool-invocation; LP-extension-action-schema-scope
---

QA impact 2026-09-04: added for the Profile selector regression on operator tool and toolset
projections. The targeted isolated walk must prove owning Profile/Workspace visibility, default and
peer isolation, public CLI/API parity, approval-token Profile/Workspace binding, and clean teardown.

QA result 2026-09-04: passed against code commit
`40587df371436a9c239dd9f6d4b76b4419af6690`. The isolated public-surface walk observed exactly two
QA Lab tools in the owning `work` Profile and candidate Workspace through both CLI and HTTP, zero in
the peer Workspace and default Profile, a completed owning-scope action with clean source identity,
and denied peer/default info and invoke calls. Race-enabled handler coverage separately proved that
an approval token cannot cross either Profile or Workspace and remains single-use.
