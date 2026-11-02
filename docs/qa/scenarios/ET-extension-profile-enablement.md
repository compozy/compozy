---
id: ET-extension-profile-enablement
area: ET
title: Enable an extension independently per profile
persona: Bruno
journey: J-adopt-extension-profiles
expected: A new install is enabled by default in every active profile; disabling it in marketing hides only marketing resources, leaves finance live, and persists as one exception row; CLI, HTTP, UDS, native tools, web detail, inventory, and command-palette projection show the same effective state.
entry_points: /marketplace/extension/{entry_id}; compozy --profile <name> extension enable|disable; GET|PUT /api/extensions/{name}/enablement; compozy__extensions_enable|disable
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-ext-kit-enable; ET-web-extensions-manage; ET-extension-palette-contributions
---

Flagged by profiles Task 09. The final profiles QA cycle owns the first walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.


## QA impact and focused evidence 2026-11-02 — responsiveness audit

Batched tool visibility reads current profile exceptions on each request. Disabling an extension must immediately hide that profile's tools, reenable must restore them, and another profile's visibility must remain independent. Workspace inventory now gathers attachment names once before applying the existing profile projection.

Focused verification passed under `-race`: `TestExtensionToolProviderDispatch/Should reread attachment reach and profile enablement for every catalog` (`internal/extension/tool_provider_test.go`) and `TestManagerProjectsLiveResourcesByProfile` (`internal/extension/manager_test.go`), within the selected extension suites' 6.804 s receipt (`.cache/performance/extension-marketplace/extension-race-final.txt`). These use the real SQLite registry and verify effective visibility rather than cached configuration.

Existing root CI mappings, not rerun by this worker: `TestExtensionCommandRoundTripIntegration/Should require install consent and round-trip profile enablement` (`internal/cli/cli_integration_test.go`), and `E2E-023 / E2E-030: profile setup and enablement update extension details and placed palette commands` (`web/e2e/__tests__/extensions.spec.ts`).

The prior skipped live scenario remains skipped; focused backend checks do not establish a new Web, native-tool or transport-parity verdict. Full commands and limits: [extension/marketplace audit](../reports/2026-11-01-performance/extension-marketplace.md).
