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
`bun install --frozen-lockfile` restored the dependencies; `make codegen codegen-check` then passed.
Bun 1.4.4 changed a minified local variable in the unrelated OpenDesign artifact; regenerating and
checking it with the repository-pinned Bun 1.4.0 preserved the committed bytes. No generated contract
drift remains. The final CI/reviewer result is recorded on PR #1152.

## PR review follow-up

Greptile identified that checkout-local agents could no longer discover the retained `cy-create-spec`.
Postinstall now links the extension source into `.agents/skills`; the existing mirror exposes it to
Claude Code as well. The generated local link is ignored. The deleted workspace skill is not restored.
Validation runs the real installer twice and checks both links resolve to the retained source.

The first post-rebase CI lint job failed while downloading `github.com/nishanths/exhaustive` from the
Go module proxy (HTTP/2 INTERNAL_ERROR), before lint executed. GitHub refuses retry until that
workflow run finishes. The current-head workflow will provide the final lint result.

## Current-head CI repairs

- Skills-disabled boot still publishes extension skill resource records, but has no skill reconciler.
  The publisher now skips that disabled runtime's trigger, matching projector registration. Other
  resource reconciliation failures remain errors. Existing feature-flag, workspace-hook, model-catalog,
  and resource-wiring boot suites passed with only the inherited restart ID removed from test processes.
- The catalog distribution E2E expectation is now 18 (one listed existing extension and 17 MCP
  packages), matching the source catalog and the approved Batuta removal. CI owns the E2E rerun.
- Snapshot-profile and automation-run ownership migration regressions belong to global persistence.
  They now live in the existing `globaldb` migration suite and reuse its real historical prefix
  databases. Their original data checks, SQLite integrity/foreign-key checks, and three-minute
  deadlines remain. The generic stream suite still owns fresh replay, reopen, ahead and schema
  equivalence; it no longer repeats the historical setup for these two domain regressions.

The original migration regression was reproduced locally under race: both cases exhausted their
180-second deadline replaying migration 86 while constructing the pre-171 database, before the
snapshot-profile migration under test. The canonical-suite change removes that repeated fixture
construction, while retaining real historical databases and all upgrade assertions.

The consolidated regressions passed with race:
`go test -race ./internal/store/globaldb -run 'TestGlobalAutomationRunProfileMigrationTail|TestGlobalAgentSnapshotProfileMigrationConvergesUniqueKeyShapes' -count=1` (136.199s including suite setup).
The skills-disabled boot checks passed in 7.683s with
`go test -exec 'env -u COMPOZY_INTERNAL_RESTART_OPERATION_ID' ./internal/daemon -run 'TestWorkspaceRegistrationRefreshesHookBindings|TestBootInjectsComposedAssemblerForFeatureFlagCombinations|TestDaemonModelCatalogWiring|TestAgentDefinitionSyncRuntimeWiring' -count=1`.
The old head's web window-performance E2E exceeded its unchanged 50ms budget by 3ms. Its gate stays
intact; the new-head CI run determines whether that failure reproduces.
