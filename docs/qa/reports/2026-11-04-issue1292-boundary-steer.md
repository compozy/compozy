# Tool-boundary steering verification

Scope: GitHub issue #1292. Authored and verified by Codex in the issue worktree.

## Claude capability diagnosis

A real Claude ACP 0.100.0 process returned `InitializeResponse._meta.steering.supported: true`.
Its `session/new` succeeded. The current production ACP driver negotiated `steer_ext` through
initialization and session setup. During a foreground tool, `Driver.Steer` returned `injected`,
and the same turn answered `STEER_ACK_1292`. No cancel was requested before cleanup.

The reported historical `none` sessions were not supplied with their original initialize receipts.
The current normal session path already consumes the top-level announcement, so this run does not
attribute those historical sessions to a stale cache or adapter. The disposable authentication
initialization did drop `_meta`; it now preserves the announcement, with a regression at that owner.
The ordinary startup regression also explicitly exercises the Claude provider identity.

The supervised live probe, raw initialize capture and lab manifest remain in the isolated QA output. No credentials or account metadata are
included in this report.

## Boundary behavior

`TestManagerBoundarySteer` exercises the production session manager and real SQLite queue with
only the provider I/O stubbed: duplicate starts, two overlapping tools, no cancellation after the
first result, cancellation after the last result, timeout escalation, no-event after-turn delivery,
zero-timeout compatibility, and immediate explicit interrupt. Tool lifetimes are tracked before batching; only a waiting boundary forces the completion event
to flush durably before cancellation. Ordinary provider bursts retain batched persistence.

The durable acceptance retains `pending_boundary`; settlement changes the queued row to
`interrupt_fallback` without rewriting the idempotent admission receipt. The migration suite
owns preservation of preexisting guidance through upgrade and reopen. The queue-reservation suite
owns round-trip/replay of both new delivery values. Existing stop-ladder and promotion tests select
zero timeout when their invariant is immediate cancellation; their assertions remain unchanged.

## Verification

- PASS: real Claude initialize, session creation and injected guidance during a tool.
- PASS: focused ACP/config race tests for announcements, overrides, authentication and timeout validation.
- PASS: focused session race tests for boundary behavior, legacy immediate fallback and promotion replay.
- PASS: real ACP subprocess fallback integration with race, including kill/rebind and input ordering.
- PASS: canonical migration sequence, integrity and schema-equivalence checks.
- PASS: root Turbo Web typecheck; 16 Web busy-input/provenance tests; changed-file Web lint.
- PASS: `make codegen` and the root Turbo `codegen-check` dependency.

Two earlier overbroad local race selections exceeded their command timeouts on this shared host.
They are not passing receipts. The final bounded selections use anchored test names and limited
parallelism. The first codegen attempt encountered an uninstalled dependency patch; a frozen Bun
install restored the lockfile's patch and subsequent codegen passed.

The migration-row preservation rerun and focused Go lint both passed. Claude Opus 5.5/high reviewed the implementation and its corrective changes. Delivery still requires
all current-head PR checks and external reviews. Broad gates and E2E remain CI-owned. No interactive browser walk is claimed; Web state presentation has typecheck,
read-model tests, and scoped lint evidence.

The targeted provider QA auditor passed, and owned-lab teardown reported `TEARDOWN_ALL_CLEAN=true`.

## Independent review dispositions

Claude Opus 5.5/high reviewed the full diff. The native tool output enums now include both additive
states and are checked against the canonical delivery vocabulary in the existing descriptor suite.
Boundary observer failures now log without failing the active turn; a transient queue-read failure
is exercised with the timer backstop. Tool-event support persists across turns of the same runtime
and resets on process replacement. Closed tool IDs remain terminal despite later metadata updates.
The expanded boundary suite and native descriptor suite passed with race.

Tool lifetimes are observed at ingress. Only a waiting boundary bypasses the 25 ms chunk coalescer;
ordinary tool bursts retain batched writes.
Fallback protection also applies when native injection is refused or unavailable for a particular
message; prediction reflects the advertised capability, while the acceptance receipt remains
final delivery truth. Explicit interrupt cancels immediately and atomically supersedes older pending-boundary or released
fallback steering in the replacement admission transaction, while ordinary queued input is retained.
Both admission paths share that transaction; an insert failure rolls back the supersession. Terminal
markers are emitted even when the target turn ends before activation, and superseded Web provenance
remains terminal when an older delivery marker arrives late. The expanded manager suite
covers this overlap. The boundary cases and the affected fallback/promotion suites passed
with race after that correction (23.182s).
The existing no-tool-events browser fixture now requires `after_turn` and the corresponding inline
feedback, rather than accepting an open-ended delivery list. Its E2E execution is CI-owned.

Initial implementation commands (passed before PR remediation; current-round receipts follow):

```sh
CGO_ENABLED=1 go test -race ./internal/session -parallel=4 -run '^(TestManagerLiveSteerDelivery|TestManagerBoundarySteer|TestManagerBusyInputActivationCleanup|TestManagerPendingInputPromotionReplay)$' -count=1 -timeout=60s
CGO_ENABLED=1 go test -race ./internal/store/globaldb -parallel=2 -run '^TestGlobalDBSessionInputQueueGeneration$/^Should_atomically_supersede_undelivered_steering_when_admitting_an_interrupt$' -count=1 -timeout=60s
CGO_ENABLED=1 go test -race ./internal/tools/builtin -run '^TestBuiltinNativeDescriptors$' -count=1 -timeout=60s
CGO_ENABLED=1 go test -race ./internal/acp ./internal/config -parallel=4 -run 'Test(AuthenticationSteerCapability|StartCapturesSteerCapability|SessionBusyInputConfigDefaultsAndValidation|SteerCapabilityResolution)' -count=1 -timeout=60s
CGO_ENABLED=1 go test -race -tags=integration ./internal/session -run TestManagerIntegrationSteerFallback -count=1 -timeout=90s
CGO_ENABLED=1 go test -race ./internal/store/globaldb -parallel=2 -run '^(TestOpenGlobalDBReopenPreservesRowsAndStatus|TestGlobalDBSessionPromptAdmission)$/^(Should_preserve_queued_guidance_through_the_boundary_delivery_migration|Should_reserve_steering_once_and_replay_its_resolved_delivery_atomically)$' -count=1 -timeout=60s
go test ./internal/store -parallel=2 -run 'Test(ProductionMigrationStreams|MigrationSchemaEquivalence|ApplyMigrationStream|ApplyRejectsAtlasSumDrift)' -count=1 -timeout=60s
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --allow-parallel-runners --timeout=4m ./internal/acp/ ./internal/config/ ./internal/session/ ./internal/store/ ./internal/store/globaldb/ ./internal/tools/builtin/
make codegen
bunx turbo run typecheck --filter=./web
bunx turbo run test --filter=./web -- src/systems/session/lib/__tests__/session-busy-input.test.ts src/systems/session/lib/__tests__/steer-provenance.test.ts --maxWorkers=2
```

Changed-file Oxlint passed for the Web consumers and E2E contract. The test-shape checker found no
new diagnostics against the base version; preexisting whole-file diagnostics were left outside scope.
The temporary provider probe was removed from the checkout after its source was retained in the lab.

Review follow-ups corrected interrupt/parked-steer ordering, atomicity, completed-turn markers, and
stale delivery-table and configuration-lifecycle prose. The documented `after_turn` ordering remains
queue-like, as requested; newer steering may supersede undelivered guidance.

## PR review and CI remediation

Greptile and CodeRabbit identified a missing rowid copy and delayed native refusal path. The
unreleased migration now preserves rowids, including gaps and saved Goal cutoffs. Delayed native
refusals use the same boundary decision as initial refusals and retain their original acceptance
receipt. The OpenAPI compatibility suite records only the two approved additive delivery values;
its exact comparisons remain intact.

The cancellation claim shares the tool-observation lock with settlement. A tool observed before
that claim postpones cancellation until the next safe boundary or the timeout. Timeout reads and
settlement retry transient storage failures without replaying a committed cancellation. Positive
fallbacks are staged at a boundary even when the observed tool set is currently empty, so the
fenced activation can cancel immediately or notice a newly observed tool safely.

Operator interrupts, including empty interrupts, retire queued or reserved human steering without
removing ordinary queued input. The additive `superseded_by` migration persists that relationship.
Session activation repairs missing transcript markers using stable event IDs, with bounded queue
pages and idempotent ledger reads. Existing manager/store suites cover crash-window repair,
reserved-steer fencing, empty interrupts, timeout retry faults, and rollback's specific constraint.

CI also exposed two mention fixtures that kept their active turn open while waiting for after-turn
delivery; they now finish that turn before asserting the same mention resolution. The catalogue
fixture now waits for every expected context refresh before shutdown can cancel admission, with
its count and no-parked-target checks retained. The provider-burst test's assertions remain unchanged.
The SQLite paired experiment showed no throughput regression, but one retention sweep took 32.1s
against its 30s limit; the threshold remains unchanged for the next CI round.

Current-round receipts:

- PASS: `make codegen` and `make codegen-check`, including the appended supersession migration.
- PASS: real ACP subprocess fallback integration with race (9.664s) after cancellation claim refactoring.
- PASS: exact OpenAPI compatibility comparison (three runs, 46.200s) and catalogue fixture (three runs, 4.509s).
- PASS: queue-generation suite including reserved-steer fencing, rollback and persisted supersession (100.860s).
- PASS: focused interrupt/fence and idle-boundary rerun with race (23.915s).
- PASS: final combined boundary selection (35.481s), preservation/settlement selection (62.623s), and scoped Go lint (zero diagnostics).
- PASS: manager regressions for reserved steering superseded inside the native call (6.346s), covering injected, pending and unsupported outcomes.
- PASS: native-delivery regression (10.185s) and session lint (zero diagnostics) after the final terminal-outcome correction.

The earlier combined boundary runs passed every case except the idle-boundary timing assertion;
that case now explicitly requires immediate activation and is excluded from the timer lower bound.
The timer cases retain the lower-bound assertion. Invalid generated SQL discovered during the first
late-refusal rerun was corrected in the owning query and regenerated; those failed runs are not
passing evidence.

The final Opus corrective review accepted cancellation fencing, timeout retries, empty interrupts,
restart repair, migration preservation and source size. Its remaining finding was a permanent retry
when an interrupt canceled a reserved steer during the native call. Late resolution now recognizes
the persisted supersession as terminal, ends its watcher, and avoids recording canceled guidance as
delivered. The canonical manager suite covers all three native return outcomes.

## Reserved-receipt and optional-queue follow-up

The next CI round exposed an optional input-queue dereference during session activation. Managers
without a persistent input queue now skip supersession recovery, and their empty interrupts retain
the original direct cancellation path. Existing coordinator, delete, extension and heartbeat suites
own those startup paths; their assertions were kept intact.

A new Greptile finding identified that a superseded reserved steer's admission stayed
`dispatch_committed`, preventing an exact retry despite the successful first return. Supersession now
completes only that unfinished receipt in the same transaction, retaining the acceptance fields and
materializing its previous-turn identity. Already-completed delivery receipts remain unchanged.
The canonical store suite covers typed/empty interrupts and rollback, while the manager compares the
entire first and replayed response (apart from `replayed`) for all three native return outcomes.
Focused race checks passed for session (12.739s), store (11.691s) and extension (10.302s); scoped lint
reported zero issues. The heartbeat selection passed with race (7.877s), as did cancellation without
a queue (3.778s). The real ACP idle-completion integration that failed on the absent queue passed
with race (4.046s). `make codegen-check` passed after regenerating the new receipt query.

CodeRabbit's follow-up tightened the catalogue test to require exactly one refresh in each expected
execution context, retaining the source, force and no-extra-refresh assertions. Ten race repetitions
passed (5.710s). Comments now document the changed runtime functions' ownership and persistence rules.
The catalogue adjustment repairs a CI scheduling race discovered while delivering this branch; it
does not change catalogue production behavior or weaken its parked-target invariant.

The second SQLite experiment on `a7b9164d6b3a868066d5f1cc47f3b087ba3f4e1c`
([job](https://github.com/compozy/compozy/actions/runs/41938164961/job/126951819630))
preserved all 830,020 rows and the all-column hash. It reported zero operation errors, no throughput
regressions and retention sweeps of 18.8–23.9s, within the unchanged 30s limit. Acceptance still failed:
three telemetry background transactions held for 1.023–1.124s, exceeding the one-second diagnostic
threshold. Their measured commit times were 467–942ms. This is not a passing load receipt or a proven
flake; the next head must pass the same CI acceptance criteria.
