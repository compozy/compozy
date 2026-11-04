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

Live probe evidence is retained by the supervised terminal `term-84ece0d2b9dd`; the raw initialize
capture and lab manifest remain in the isolated QA output. No credentials or account metadata are
included in this report.

## Boundary behavior

`TestManagerBoundarySteer` exercises the production session manager and real SQLite queue with
only the provider I/O stubbed: duplicate starts, two overlapping tools, no cancellation after the
first result, cancellation after the last result, timeout escalation, no-event after-turn delivery,
zero-timeout compatibility, and immediate explicit interrupt. Tool lifecycle events bypass chunk
coalescing so cancellation is requested while processing the completion event.

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
- PASS: root Turbo Web typecheck; 15 Web busy-input/provenance tests; changed-file Web lint.
- PASS: `make codegen` and the root Turbo `codegen-check` dependency.

Two earlier overbroad local race selections exceeded their command timeouts on this shared host.
They are not passing receipts. The final bounded selections use anchored test names and limited
parallelism. The first codegen attempt encountered an uninstalled dependency patch; a frozen Bun
install restored the lockfile's patch and subsequent codegen passed.

The migration-row preservation rerun and focused Go lint both passed. Delivery still requires
the requested Claude Opus 5.5/high review and all current-head PR checks and external reviews. Broad gates and E2E
remain CI-owned. No interactive browser walk is claimed; Web state presentation has typecheck,
read-model tests, and scoped lint evidence.

The targeted provider QA auditor passed, and owned-lab teardown reported `TEARDOWN_ALL_CLEAN=true`.

## Independent review dispositions

Claude Opus 5.5/high reviewed the full diff. The native tool output enums now include both additive
states and are checked against the canonical delivery vocabulary in the existing descriptor suite.
Boundary observer failures now log without failing the active turn; a transient queue-read failure
is exercised with the timer backstop. Tool-event support persists across turns of the same runtime
and resets on process replacement. Closed tool IDs remain terminal despite later metadata updates.
The expanded boundary suite and native descriptor suite passed with race.

Tool events still bypass the 25 ms chunk coalescer to preserve the requested immediate boundary.
Fallback protection also applies when native injection is refused or unavailable for a particular
message; prediction reflects the advertised capability, while the acceptance receipt remains
final delivery truth. Explicit interrupt cancels immediately and supersedes older pending-boundary steering so its
replacement text runs next, while ordinary queued input is retained. The expanded manager suite
covers this overlap. All eight boundary cases and the affected fallback/promotion suites passed
with race after that correction (23.182s).
The existing no-tool-events browser fixture now requires `after_turn` and the corresponding inline
feedback, rather than accepting an open-ended delivery list. Its E2E execution is CI-owned.

Final local commands (all PASS on the reviewed source):

```sh
CGO_ENABLED=1 go test -race ./internal/session -parallel=4 -run '^(TestManagerLiveSteerDelivery|TestManagerBoundarySteer|TestManagerBusyInputActivationCleanup|TestManagerPendingInputPromotionReplay)$' -count=1 -timeout=60s
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

The second review identified and corrected this interrupt/parked-steer ordering gap plus stale
delivery-table and configuration-lifecycle prose. The documented `after_turn` ordering remains
queue-like, as requested; newer steering may supersede undelivered guidance.
