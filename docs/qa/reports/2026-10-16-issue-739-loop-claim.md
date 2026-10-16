# Issue #739 — Loop claim ownership and recovery

## Scope and evidence

Issue: https://github.com/compozy/compozy/issues/739. Base: `2cf9f5fae89db8c0e5ff1db61e108b7ea17464e0`.
The report concerns non-reviewed `run-agent` nodes stranded in queued after a provider completes,
a missing claim-token hash, and unsuccessful operator recovery.

Confirmed from source and deterministic regressions:

- `task.Service.ClaimNextRun` commits its claim and audit before returning. The daemon calls
  `ExecuteActionRun` only after that successful return. Neither investigated historical commit
  dispatches a provider from an uncommitted claim. `a4b37c255` bounds scheduled automation claims;
  `2cf9f5fae` allocates binding epochs using the action-cell fence, including non-reviewed actions.
- `ReleaseRunLease` and `BlockTaskAndReleaseRun` omitted the existing daemon-owner authority guard.
  The bound worker could clear the claim while its daemon executor retained the original token.
  Completion would then reach `requireCurrentRunLease` with no hash. The regression rejects both
  operations and then completes through the owning daemon, proving node success remains possible.
- A failed SQLite claim attempt had no persistence retry in the action runtime. A bounded failed
  attempt could leave queued work without another dispatch. The existing SQLite retry helper now
  surrounds only claim persistence; provider execution is outside the retry callback.
- Forced task failure did not publish the corresponding Loop node failure or terminal hook.
  The existing terminal-output transaction and post-commit hook now carry that recovery into the
  coordinator. Run recovery remains needs-attention-only; node requeue remains quarantine-only.
- Transactional run reads omit profile identity; the owning task supplies it to terminal hooks.
  Mechanical/coordinator task runs without sessions have no session resource to release and are
  skipped by the terminal-session observer. Session-backed incomplete identities still fail.

The original reporter's task event ledger was not provided. Worker release is a demonstrated
failure mechanism, not a claim that historical runs definitely used that operation. No live user
state, database, daemon, desktop or provider configuration was changed during this investigation.

## Test ownership

- Store: `TestGlobalDBRunLeaseTerminalShouldRecordLoopNodeProgress` owns daemon claim protection,
  successful completion, and queued/claimed force failure committing the node's terminal output.
- Daemon: `TestLoopGoalManagedRuntimeIntegration/Should_wait_for_a_durable_claim` uses real SQLite,
  a non-reviewed isolated `run-agent` node and a deterministic writer barrier. The first claim
  times out, no action executes without a persisted claim, retry executes once, and node success
  commits. The provider boundary is an injected action executor; no full daemon lab is started.
- Service: `TestManagerForceRunOperations/Should_publish_Loop_worker_failure` observes the failed
  run from inside the post-commit terminal hook; `TestTaskRunHookContextCarriesLoopFilterKeys`
  checks the owning profile.
- Terminal adapter: `TestTerminalRunLifecycleObserverShouldReleaseCurrentSessionGeneration`
  checks current generation release and legitimate sessionless terminal events.

## Validation

Passed local, race enabled:

```sh
CGO_ENABLED=1 go test -race -run TestGlobalDBRunLeaseTerminalShouldRecordLoopNodeProgress ./internal/store/globaldb
CGO_ENABLED=1 go test -race -run 'TestGlobalDBBlockTaskAndReleaseRun|TestGlobalDBRunLeaseTerminalShouldRecordLoopNodeProgress|TestGlobalDBTokenFencedLeaseLifecycle' ./internal/store/globaldb
CGO_ENABLED=1 go test -race -tags integration -timeout 90s -run 'TestLoopGoalManagedRuntimeIntegration/Should_wait_for_a_durable_claim' ./internal/daemon
CGO_ENABLED=1 go test -race -run 'TestTaskRunHookContextCarriesLoopFilterKeys|TestTokenFencedLeaseTransitionsDispatchTaskRunHooks' ./internal/task
CGO_ENABLED=1 go test -race -run 'TestTerminalRunLifecycleObserverShouldReleaseCurrentSessionGeneration|TestLoopActionRuntimeRetriesWorkspaceCapacityDeferral|TestLoopActionRuntimeEnforcesActionLiveness' ./internal/daemon
```

A read-only Go overlay of the original production files reproduced failed regressions:
worker release returned nil rather than permission denied, and forced failure of both queued
and claimed workers left their node outputs `enqueued` rather than `failed`. The checkout was
never reset or restored. The claim-contention baseline also failed at the first SQLite admission deadline, before executing
the action; the corrected runtime passed after retrying that same persistence operation. Final
lint/CI receipts are recorded below when complete.

The test-shape checker passes both changed integration files. Its findings in the large store,
hook and manager suites are unchanged from the base (8, 1 and 31 respectively); notifier findings
reduce from 2 to 1. These unrelated existing test-shape findings were left untouched.

An initial contention fixture waited at the wrong boundary and hung; its own `daemon.test` process
was terminated after collecting its stack. The corrected barrier is inside the exact claim
attempt, with a 90-second test deadline; the focused test passes in 6.632 seconds.

Per maintainer instruction, `make gate`, full Go suites, full Turborepo suites, E2E suites and
full-daemon QA labs were not run locally. Required PR CI, including the managed runtime integration
lane, is the delivery gate. No UI implementation changed; screenshots are not applicable.

## Delivery receipts

Pending: scoped lint, final PR head CI, CodeRabbit, Greptile and React Doctor.
