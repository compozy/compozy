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

## Startup and shutdown follow-up

Main CI `34446647506` passed completely at `e368da936e3cd61e9c94900d2fcc2c70eae30895`, including Windows terminal timing, every Go shard, and desktop/runtime/Web E2E. The automatically refreshed PR head `f8969201def9143c8b2605ee35fdd9e168fc58a9` exposed three remaining paths:

- CLI lease integration completed its assertions but timed out draining the UDS server. `waitForDaemonStop` issued full HTTP status requests after signaling shutdown even though process identity and liveness exclusively determine completion. An unresponsive status API exhausted the wait context after process exit in the existing daemon-wait suite. The CLI now observes that same process identity without adding requests to the draining server. The regression failed before repair (`/tmp/compozy-cli-stop-unresponsive-before.log`); daemon-stop tests passed 30 repetitions after repair (`/tmp/compozy-cli-stop-unresponsive-after.log`). The complete real CLI integration suite passed in 92.538 seconds (`/tmp/compozy-cli-full-integration-stop-repair.log`). No shutdown budget or assertion was relaxed.
- The orchestrated Profile extension Agent failed its initial durable process registration under the separate one-second process-record deadline. Registration now uses its startup context, like process launch and ACP initialization; interruption and exit records retain their independent bounded contexts. The ACP process lifecycle suite owns startup-budget and cancellation regressions: both failed before repair (`/tmp/compozy-acp-register-startup-before.log`), and the affected lifecycle cases passed ten repetitions after repair (`/tmp/compozy-acp-register-startup-after.log`).
- The full integration runner lacked built TypeScript and React SDK exports for the programmable-view fixture. Its workflow now invokes the existing root Turborepo dependency build before tests, matching the E2E asset preparation. A forced SDK build passed (`/tmp/compozy-extension-fixture-dependencies-build.log`), as did workflow validation (`/tmp/compozy-release-sdk-actionlint.log`). The Profile extension Agent journey and programmable-view isolation/restart journey passed three real race-enabled repetitions after the repairs (`/tmp/compozy-release-lifecycle-e2e-repair.log`, 55.780 seconds).

The changed ACP and CLI packages cross-built for Windows (`/tmp/compozy-lifecycle-windows-build.log`). Change impact remains owned here: native tool IDs, public CLI output, hooks/configuration, workspace/profile data and schema are unchanged. Startup and CLI stop use their existing lifecycle surfaces; official skill syntax remains valid. No Web UI behavior or browser result is inferred from these targeted backend checks. The next main/PR checks, merge, and publication remain required.

## Interrupted identity, progress bursts, and browser port allocation

PR head `2ceb6d8a224f1e3cb935d294cbb81e8bdc1f129f` passed normal CI. Release integration found a session-creation identity race during automation shutdown; nightly integration lost a tool-start progress event. Main Web CI separately encountered a fixture HTTP port collision.

- Starting-session stop now registers its complete snapshot before moving the catalog out of `Starting`. A canceled launch can persist the metadata witness without committing its catalog transaction; state-only stop persistence previously made that identity permanently unbindable. `TestSharedSessionStopOperation` owns the invariant using the real SQLite catalog. The regression failed with the missing identity before repair, then passed 20 race-enabled repetitions (`/tmp/compozy-stop-identity-red.log`, `/tmp/compozy-stop-identity-green.log`, 17.275 seconds). The real automation trigger/history/run integration passed 20 repetitions (`/tmp/compozy-automation-stop-identity-integration.log`, 64.852 seconds). The immutable-identity mismatch guard remains unchanged.
- The default bridge delivery queue now holds 64 events, allowing short tool-lifecycle bursts while an adapter request is in flight. The previous four-event capacity evicted starts during an ordinary three-tool turn. The existing `TestBrokerProgressQueueBackpressure` suite now blocks adapter I/O and verifies that all six progress events and both delivery boundaries arrive in order. Before repair only five of eight events arrived; the complete bridges suite passed after repair (`/tmp/compozy-bridge-burst-red.log`, `/tmp/compozy-bridge-burst-green.log`). Existing explicit-capacity saturation, coalescing, terminal preservation, and drop-metric tests remain intact. The real opted-in low-tier bridge journey passed ten race-enabled repetitions, including all six progress events and transcript/redaction assertions (`/tmp/compozy-bridge-burst-integration.log`, 59.628 seconds).
- The browser harness selects its HTTP port after starting fixture servers and seeding the home, preventing those servers from reusing the port it just released. Like the Go runtime harness, it retries a confirmed HTTP bind collision at most three times, only after failed-attempt cleanup succeeds. Other startup failures still fail immediately. No browser assertion or test retry setting changed. The real Marketplace extension update flow passed five repetitions (`/tmp/compozy-web-port-repair-e2e.log`, 5.1 minutes); this validates normal startup and the update journey, without claiming a forced collision exercise. Root Turborepo typecheck and all 7,116 Web tests passed (`/tmp/compozy-web-port-repair-checks.log`).

Change impact is owned here: session shutdown and bridge progress use existing public CLI/HTTP/UDS and extension delivery surfaces; native tool IDs, hooks/config keys, persisted schema, workspace/profile isolation, and official skill syntax are unchanged. Bridge consumers can receive more intermediate progress during short bursts; sustained saturation retains the existing bounded-queue policy. The browser change affects fixture startup only. Current-head CI/release checks, merge, and publication remain required.

Final local validation passed: `make gate` approved Go lint, the complete affected session/bridge suites, and Web lint/typecheck/tests (`/tmp/compozy-identity-bridge-port-final-gate.log`). Session tests took 139.386 seconds; Web lint reported zero warnings and zero errors.

## Cancellation worker ordering

Main runtime CI at `45c05df1f` and PR Go shard 7 at `f50a486c` exposed two cancellation scheduling gaps. An accepted cancellation could lose its provider signal if the prompt drained before the worker ran. A new cancellation could also replay the old receipt after the prompt had drained while that worker still finalized its journal.

The turn-stop claim now checks active-turn ownership before returning a pending receipt. The termination ladder still sends the cooperative signal for an accepted turn cancellation when the process is alive; prompt drainage alone no longer skips the request. Verified process exit continues to bypass signaling. Existing turn admission fencing keeps the delayed signal away from a later turn.

`TestCancelPrompt` owns the deterministic scheduling-gap regression. With the worker held between claim and execution, the old code returned `canceled` for the drained turn and made zero provider-cancel and scoped-interrupt calls (`/tmp/compozy-turn-cancel-order-red.log`). Existing cancellation assertions remain unchanged. The entire `TestCancelPrompt` suite passed 30 race-enabled repetitions after repair (`/tmp/compozy-turn-cancel-order-green.log`, 44.818 seconds).

Change impact: CLI `session prompt-cancel`, `compozy__session_prompt_cancel`, and their shared HTTP/UDS manager path retain existing outcomes and exit codes. Hooks/configuration, wire shapes, workspace/profile isolation, schema, and official skill instructions are unchanged; the existing instructions already specify `nothing-in-flight` after the turn ends. Web consumers receive the corrected shared cancellation behavior; targeted backend evidence does not claim a separate browser re-walk.

The initial full attention re-walk passed the cancellation journey but failed the bounded-wait outcome. Inspection identified an ambiguous setup: a generic running badge could belong to an earlier turn before the asynchronously launched holding request reached the provider. Twenty isolated bounded-wait repetitions and five subsequent full attention repetitions passed during diagnosis, so this remains a suspected setup cause rather than a deterministic reproduction. The harness now waits for the holding request's own streamed assistant event before inspecting status or waiting. The five-second timeout and exit-75 requirement remain unchanged. All attention journeys then passed ten race-enabled repetitions (`/tmp/compozy-attention-confirmed-hold-integration.log`, 258.213 seconds).

Release integration shard 7 also exposed a contradictory stubborn-process fixture: its first prompt returned when the JSON-RPC request context was canceled, although the scenario requires ignored cancellation and process replacement. The first fixture prompt now remains blocked until its process is killed. The existing real-ACP steer fallback scenario passed twenty repetitions with escalation, process identity replacement, and ordered replacement/queued dispatch assertions intact (`/tmp/compozy-steer-stubborn-integration.log`, 39.962 seconds).

## Thinking indicator deadline

Both main and PR frontend verification failed the first Goal/StrictMode cancellation journey because its pending activity indicator could remain hidden. The flicker-guard timer published its clock once; a callback that ran before the wall-clock deadline left the guard pending with no future update. The hook now re-arms the remaining interval after an early callback and preserves cancellation during cleanup/replay.

The existing `SessionThread thinking guard` suite owns the timing invariant. Its new deterministic case advances timer scheduling ahead of the wall clock under StrictMode: it failed before repair (`/tmp/compozy-thinking-early-timer-red.log`) and passed afterward together with both complete conversation/runtime suites, 189 tests (`/tmp/compozy-thinking-early-timer-green.log`). The existing delayed-effect case and first Goal cancellation assertions are unchanged. Root Turborepo Web build and typecheck passed (`/tmp/compozy-thinking-timer-web-build.log`).

Change impact: this hook only controls the existing Web thinking indicator's 250 ms flicker guard. Native tools, hooks/configuration, public wire surfaces, persistence, workspace/profile isolation, and official skill syntax are unchanged.

The existing E2E-015 browser Stop journey passed three repetitions against the freshly built Web distribution (`/tmp/compozy-cancel-thinking-browser-e2e.log`, 2.6 minutes). It retained stopping-state, double-click, draft, and interrupted-turn assertions. React Doctor scanned the changed Web files with a score of 100/100 and no issues (`/tmp/compozy-thinking-react-doctor.log`). This is controlled-runtime browser evidence, not external-provider validation.

## Lineage migration verification contexts

Release integration shard 6 at PR head `f50a486c` exhausted a five-minute context on the post-reopen lineage query. The test created that context before upgrading the historical database, then reused it after two independently budgeted database opens and their shared migration admission waits. The existing lineage migration/reopen suite now starts a fresh bounded verification context after each open, matching the neighboring migration suites. Default notification, persisted opt-out, complete migration history, and reopen assertions remain unchanged. No production timeout, schema, migration bytes, or query was changed.

The owning real SQLite migration/reopen suite passed five race-enabled repetitions (`/tmp/compozy-lineage-migration-context-integration.log`, 200.550 seconds).

The Go test-shape heuristic passed the attention and lineage files. Its findings in the two session files concern existing untouched tests and helper-process entry points; the added cancellation subtest follows the owning suite's existing shape. Current-head CI/release checks, merge, and publication remain required.

## Catalog rehydration fixture freshness

The final gate exposed a race between the curation fixture's two catalog reads. The first read starts background discovery; the seeded live source had no next-refresh deadline and its intentionally unavailable provider immediately failed discovery. The second read excludes stale rows by contract. A diagnostic refresh reproduced the failure and an include-stale read found the same persisted row with `Stale=true` (`/tmp/compozy-model-rehydrate-stale-diagnostic.log`). This establishes filtering after discovery failure rather than deletion during static rehydration.

The existing `TestDaemonModelCatalogWiring` rehydration fixture now seeds a fresh live discovery status, so background discovery cannot change the row's freshness between its reads. The builtin/config metadata remains the pre-curation input; all curated, default-only, and live-only assertions are unchanged. Diagnostic calls were removed after establishing the cause. Production discovery, stale filtering, persistence, and API behavior are unchanged.

The unchanged rehydration assertions passed ten race-enabled repetitions after fixture repair (`/tmp/compozy-model-rehydrate-fresh-integration.log`, 67.689 seconds).

The complete daemon race suite passed after the fixture repair (`/tmp/compozy-daemon-catalog-fresh-final.log`, 154.026 seconds). Root Turborepo Web lint/typecheck and all 7,117 tests passed (`/tmp/compozy-cancel-thinking-web-final.log`, 2 minutes 42.713 seconds). The earlier gate invocation retains its daemon failure; these are subsequent successful checks, not a relabeling of that run.

The complete affected persistence suites passed, including `globaldb` in 879.689 seconds. Final `make gate` passed Go lint, the affected race suites, code generation verification, and Web lint/typecheck/tests (`/tmp/compozy-cancel-thinking-lineage-repaired-gate.log`). Go lint and Web lint reported zero issues. Remote current-head checks and release publication remain outstanding.
