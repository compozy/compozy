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
evidence: docs/qa/reports/2026-09-09-pr-551-profile-tool-isolation.md
last_report: docs/qa/reports/2026-09-09-pr-551-profile-tool-isolation.md
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

QA result 2026-09-09: repeated the public CLI/HTTP/extension walk after rebase; PASS. See the latest report for receipts and scope limits.


## QA impact and focused evidence 2026-11-02 — responsiveness audit

The extension catalog now resolves current profile enablement, attachment reach and workspace development generation in one request-scoped SQLite query. No authorization/configuration TTL was added. Selected-profile and exact-workspace isolation must remain unchanged after attach, detach, disable, reenable and development unlink.

Focused verification passed under `-race`: `TestExtensionToolProviderDispatch` in `internal/extension/tool_provider_test.go`, including `Should reread attachment reach and profile enablement for every catalog` and `Should expose profile-placed tools only in the named profile`; `TestManagerProjectsLiveResourcesByProfile` in `internal/extension/manager_test.go` covers live resource projection. The selected extension suites completed in 6.804 s (`.cache/performance/extension-marketplace/extension-race-final.txt`).

Existing root CI mappings, not rerun by this worker: `TestCLIToolCommandsMatchUDSContractsIntegration` (`internal/cli/tool_integration_test.go`); `TestToolHandlersProjectSelectedProfileCatalog` and `TestToolApprovalHandlersMintAndConsumeSingleUseTokens` (`internal/api/core/tools_test.go`); `TestToolMCPStaticPublicationAndBootRebuild/Should publish explicit profile installations from the real manager` (`internal/daemon/tool_mcp_resources_integration_test.go`). The browser transport/approval owners are `operator installs a local extension tool provider, invokes it over transports, and verifies fail-closed manifest security` and `operator approves and denies tool permission prompts with accessible browser controls and transport evidence` in `web/e2e/__tests__/marketplace.spec.ts`; they do not replace the selected-profile isolation assertions.

This is focused manager/provider evidence, not a new CLI/HTTP/browser scenario walk. Historical verdicts above remain unchanged. Commands, measurements and limitations: [extension/marketplace audit](../reports/2026-11-01-performance/extension-marketplace.md).
