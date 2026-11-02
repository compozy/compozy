# QA and PR remediation

Scope: findings 2, 3 and the unused Batuta archive in finding 6 of QA.md; PR #1152.
The permission ceiling decision in finding 1 remains unchanged.

## Changes and owning checks

- Missing action tools terminalize failed Loop generations instead of reattempting into quarantine.
  Invariant: a removed step fails only its run. Owner: Loop coordinator;
  `TestCoordinatorRunnerShouldIsolateRepeatedFailures/Should_fail_a_removed_action_without_reattempt_or_quarantine`.
  Existing healthy-sibling quarantine coverage remains unchanged.
- Durable action errors identify the absent ToolID and explain recovery without exposing internal errors.
  Owner: daemon action failure boundary; `TestLoopActionFailureMetadataShouldPreserveSafeOperatorDetail`.
- Missing agents return HTTP/UDS 404 through the shared session error mapper.
  Owner: API core; `TestSessionWorkspaceStatusMappings`.
- Attachment refusals identify unavailable agents or the persisted session state.
  Owner: session manager; `TestManagerLifecycleCatalogTransitions`, stopped and removed-agent cases.
- Catalog regenerated with `go run ./cmd/compozy-catalog publish ./catalog ./catalog`,
  then the unreferenced Batuta archive removed and `go run ./cmd/compozy-catalog validate ./catalog` passed.
- CI formatting failures repaired in the four named Markdown/MDX files.
- CI complexity failure repaired by extracting saved-trigger target validation without changing admission.
  Existing automation resource sync suites own validation and persistence behavior.
- Goal tests retain the exact former output schema as a local fixture, removing their dependency on a
  deleted bundled Loop. The authoritative result and prompt contract assertions remain intact.

## Verification

Passed focused tests in `internal/loop`, `internal/api/core`, `internal/daemon`, and `internal/session`:
`-run 'TestCoordinatorRunnerShouldIsolateRepeatedFailures|TestSessionWorkspaceStatusMappings|TestLoopActionFailureMetadataShouldPreserveSafeOperatorDetail|TestManagerLifecycleCatalogTransitions'`.
The removed-agent attachment cases also passed separately.

Passed real daemon/SQLite upgrade integration:
`COMPOZY_INTERNAL_RESTART_OPERATION_ID= TMPDIR=/private/tmp go test -tags=integration ./internal/daemon -run TestRemovedResourceUpgradeIntegration -count=1`.
An initial run inherited the host restart operation ID and failed before fixture boot; clearing only
that variable for the isolated test process resolved it.

Passed automation/Goal tests:
`go test ./internal/automation ./internal/loop/goal -run 'TestAutomationResourceSyncManagedDefinitions|TestManagedDefinitionSync|TestExecutorShouldProjectAuthoritativeTerminalStatus|TestAuthoritativeGoalStructuredResultShouldSatisfyOutputContract|TestExecutorShouldRestorePriorBlockingIssuesAfterRestart' -count=1`.

Pinned golangci-lint v2.15.0 passed on the QA diff in loop, API core, daemon and session.
Formatting of the four CI-reported Markdown/MDX files passed.
Test-shape checker findings are pre-existing, outside edited cases (inline assertions in existing
functions and one existing discarded error in Goal tests); no new case triggers a finding.

Pinned lint on automation and Goal also passed with zero issues. The focused automation/Goal
command above also passed with `-race` and `CGO_ENABLED=1`.

Codegen initially failed because local JS dependencies lacked the locked stream cancellation patch.
`bun install --frozen-lockfile` restored the dependencies; codegen/drift checks and the final
CI/reviewer result remain pending.
