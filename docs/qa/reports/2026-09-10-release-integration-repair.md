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
- **Workspace/profile isolation:** Terminal roots derive from the resolved session environment. Automation history retains its original profile after parent deletion, and unknown legacy ownership stays inaccessible to profile-scoped reads. Migration 108 is an additive, lossless user-state upgrade; no historical migration is edited.
- **Official skill:** Existing tools and syntax remain valid. The Loop reference documents recovery from a missing credential; no new public surface is introduced.
- **Web/docs:** `/jobs`, `/triggers`, Loop run details, and extension status consume the corrected existing payloads. No Web component or DTO change. Scenario notes below link the backend evidence without replacing pending browser evidence.
