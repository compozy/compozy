# Release integration recovery — 2026-09-10

Release PR #558 exposed stale integration fixtures and runtime defects after main CI passed at `404422bd72cf02c76b7f858d23882affdf4e564d`. Release run `34428684635`, job `102719360251`, failed its full integration lane. The failure inventory contained 70 direct failures and 1,113 cancellations after package timeouts; canceled tests were not counted as independent defects.

## Runtime repairs

- Invalid extension manifests no longer abort declared-profile reconciliation. The extension remains diagnosable while healthy extensions and the daemon continue starting.
- Automation runs snapshot their profile owner. Migration 108 backfills ownership from surviving parents or session/Loop provenance, preserves all history and metadata, and retains unknown orphans with a null owner. Scoped reads fail closed for unknown owners; all-profile administration retains access. Deleting a job or trigger no longer hides its run history from its owner.
- Local terminal sessions receive the same permitted roots as their resolved worktree sandbox, allowing commands inside the selected checkout while retaining containment checks.
- Unresolved provider credentials preserve the safe `credential_missing` classification through session creation. The coordinator's existing explicit-dependency contract terminalizes the run as `blocked` before ACP spawn. Creating run-agent bindings can settle into durable cleanup; late session creation is stopped even in a root environment without a worktree.

## Fixture and execution corrections

Integration fixtures now seed explicit profile ownership, real workspace identities, durable channels, session catalogs, prompt admission/queue stores, and terminal journals. In-process drivers verify their own completion signals. Lifecycle assertions use the current waited-stop receipt, queued-input admission, transcript markers, profile extension enablement and secret fallback, and preserved orphan lineage. Native extension lifecycle tests use the real extension manager and consent flow. Provider-model curation disables live provider discovery in that fixture so external runtime metadata cannot race the transport parity assertions. The persisted-catalog rehydration unit fixture uses an unavailable provider command so a live machine installation cannot replace the seeded historical rows.

The credential integration follows `TestCoordinatorRunnerShouldClassifyExplicitDependencyFailureAsBlocked`; the former quarantine expectation contradicted that owning contract. The orphan-parent integration follows `TestCreateSystemSessionRecordsInternalProvenance/Should persist a missing provenance parent as an orphan root` in the session lineage suite. These are fixture contract corrections; production boundary validation remains enabled.

The release integration lane now uses eight shards. Every regular package is assigned once, and top-level tests in `globaldb`, `daemon`, and `store` are distributed across shards. The existing partition algorithm and census remain authoritative. Race detection, full pointer checks, four test slots, and the 30-minute per-invocation timeout remain enabled; CI runs one package at a time. Shard JSON results accompany failure artifacts.

## Verification

Existing suites own each invariant: automation profile isolation and deletion history in `global_db_automation_test.go`; migration preservation in `migrate_streams_test.go`; binding cleanup in `global_db_goal_binding_integration_test.go`; safe failure metadata and actual runtime behavior in the daemon's existing action/managed-runtime integrations; worktree containment in `daemon_worktree_e2e_integration_test.go`; shard uniqueness in `magefiles/gotest_lane_test.go`.

Completed local evidence:

- Full `globaldb` integration with race checks: 1,336.435 seconds, passed (`/tmp/compozy-globaldb-full-integration.log`).
- Migration fresh/reopen/ahead and schema equivalence: 158.549 seconds, passed (`/tmp/compozy-run-profile-schema-check.log`). Migration-tail ownership and orphan preservation passed separately (`/tmp/compozy-run-profile-migration2.log`).
- ACP, HTTP/UDS transport, CLI stop/marketplace, Telegram adapter conformance, extension lifecycle/secrets, worktree commands, and profile fixture repairs passed in their existing real integration suites. Logs are retained under `/tmp/compozy-*-integration*.log`.
- Provider-model CLI/HTTP/hosted-native parity and missing-credential classification passed (`/tmp/compozy-daemon-contract-final.log`).
- Pending binding cleanup passed (`/tmp/compozy-binding-pending-regression.log`).
- Complete managed-runtime and runtime-selection integration: 126.860 seconds, passed (`/tmp/compozy-managed-credential-final.log`), including cancellation during session creation.
- `make codegen-check` passed (`/tmp/compozy-integration-codegen-check.log`).
- Real release integration shard 0/8 passed all four invocations: 1,946 general-package tests, 221 globaldb tests, 283 daemon tests, and 40 migration/store tests (`/tmp/compozy-integration-shard0.log`; structured events in `/tmp/compozy-integration-shard0-json`).
- The complete daemon unit suite passed after controlling the rehydration fixture, 156.794 seconds (`/tmp/compozy-daemon-gate-repair.log`).
- Catalog rehydration regression passed three repetitions (`/tmp/compozy-model-rehydrate-regression.log`).
- Mage shard distribution tests passed (`/tmp/compozy-integration-shard-tests.log`).

The first full daemon run exposed the repaired residuals; it is not recorded as a pass. Final gate, current-head remote integration, merge, and publication remain delivery checks at the time of this report. Backend integration evidence does not promote unrelated historical browser scenarios or assert live external-provider behavior.

## Change impact

- **Native tools:** IDs and schemas are unchanged. Existing Loop/runtime, extension lifecycle, and terminal tools benefit from corrected execution and diagnostics; CLI/HTTP/UDS fallbacks remain unchanged.
- **Extensibility/hooks/config:** Invalid extension startup is isolated; consent, profile enablement, secret redaction, and hook lifecycle are exercised through existing integrations. No new configuration keys or compatibility aliases.
- **Workspace/profile isolation:** Terminal roots derive from the resolved session environment. Concurrent workspace registration publishes one stable identity without replacing an existing identity. Automation history retains its original profile after parent deletion, and unknown legacy ownership stays inaccessible to profile-scoped reads. Migration 108 is an additive, lossless user-state upgrade; no historical migration is edited.
- **Official skill:** Existing tools and syntax remain valid. The Loop reference documents recovery from a missing credential; no new public surface is introduced.
- **Web/docs:** `/jobs`, `/triggers`, Loop run details, and extension status consume the corrected existing payloads. No Web component or DTO change. Scenario notes below link the backend evidence without replacing pending browser evidence.

## Current-head follow-up

Main CI `34440883911` passed at `8128187b56c2b6e82cb19a026cd5e71eb5b114f5`. Release PR head `1cd1b8010b823e4ddbb9c799bc9617612934e835` exposed additional races and fixtures:

- `CreateReady` now checks caller cancellation after either completion signal wins the select. Existing cancellation tests passed 200 race-enabled repetitions; the real worktree lifecycle integration passed in 44.260 seconds (`/tmp/compozy-worktree-cancel-integration.log`).
- Automation finalization now uses a bounded cancellation-independent context even when cancellation arrives after finalization starts. The previous context could abandon clearing a completed fire's deferred cursor. The existing scheduler restart integration reproduced the defect and passed 15 repetitions after repair (`/tmp/compozy-deferred-fire-regression2.log`, 81.706 seconds). The dispatcher suite owns a deterministic context regression.
- Desktop link navigation waits for readiness and serializes document loads, retaining the latest pending link during a load. The existing packaged E2E-004 passed three repetitions after repair (`/tmp/compozy-desktop-deeplink-repair.log`); desktop typecheck and unit tests passed through root Turborepo (`/tmp/compozy-desktop-navigation-check.log`). The full packaged desktop suite and Linux current-head CI remain delivery evidence to collect.
- HTTP/UDS transcript checks identify the typed dispatch receipt independently of its position relative to agent output. They retain all seven messages and the assistant/tool assertions. Both real transports passed 30 repetitions (`/tmp/compozy-transcript-order-regression2.log`).
- CLI integration clients close their shared transport's idle connections during cleanup. The lease lifecycle passed eight repetitions (`/tmp/compozy-cli-lease-cleanup-repair.log`, 33.947 seconds).
- Provider conformance uses explicit profile ownership and the existing synchronized log buffer. Reference fixtures use the current hook descriptor/handshake types and exclude the newly bundled forge extension from this isolated reference-extension harness. Both existing extension suites passed (`/tmp/compozy-extension-conformance-repair3.log`, 17.279 seconds).
- Migration verification starts a fresh bounded context after the separately budgeted upgrade. Completion-time evidence and unknown-history assertions remain unchanged; the real migration/reopen case passed (`/tmp/compozy-loop-completion-migration-repair.log`, 15.362 seconds).

The Windows PTY job reported 202.055 ms against its existing 200 ms read-unblock limit while the same-head main Windows lane passed. No timing assertion was relaxed; the next current-head Windows run must pass. These follow-ups retain the change-impact ownership above: no new public DTOs, tool IDs, configuration, or schema. Desktop forwarding and scheduler cancellation use existing surfaces; the official skill's commands remain unchanged.

The full macOS desktop rerun passed 28 scenarios and exposed cross-test updater-cache reuse in the remaining bootstrap-failure case. A cached artifact skipped the fixture's held HTTP download and correctly canceled bootstrap for installer handoff before the expected failure marker. Each update fixture now owns a unique cache, removed during teardown; all original state and update assertions remain. The formerly failing case passed three repetitions (`/tmp/compozy-desktop-bootstrap-cache-repair.log`). An earlier local runtime-coherence failure used a stale packaged runtime; rebuilding the bundle against migration 108 restored that unchanged scenario.

All four packaged updater scenarios then passed with isolated caches (`/tmp/compozy-desktop-updates-cache-final.log`, 44.7 seconds). Together with the unchanged shell scenarios from `/tmp/compozy-desktop-navigation-final.log`, this verifies all 29 desktop scenarios against the current relevant inputs; it is not a claim that the preceding failing full invocation passed.

The full daemon integration run exercised the tests behind the release shards' earlier package failures and exposed three residual failures (`/tmp/compozy-daemon-full-integration-final.log`, 967.917 seconds). Concurrent identity creation could overwrite `workspace.toml`, returning different IDs for the same root and breaking boot/reentry lookup. Identity creation now uses the existing atomic publication primitive without replacement and loads the winning identity when another creator publishes first. The workspace persistence suite owns the concurrent-creator invariant; all identity cases passed 50 race-enabled repetitions (`/tmp/compozy-workspace-identity-public-race.log`). The existing real boot and detached-harness integrations passed three repetitions (`/tmp/compozy-daemon-workspace-identity-repair.log`, 174.118 seconds).

The attention CLI fixture now waits for stop completion before removing its deleted-target preparation session, using the existing `session stop --wait` contract. Its bounded-wait outcome assertions are unchanged; the owning real attention integration passed five repetitions (`/tmp/compozy-daemon-attention-stop-repair.log`, 50.652 seconds). The preceding full daemon invocation remains a failed run; focused repairs and unchanged passing coverage are recorded separately.
