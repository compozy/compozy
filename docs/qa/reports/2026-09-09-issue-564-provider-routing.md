# Issue #564: spawned provider command routing

## Scope and result

The Session Manager reproducer selected global command A instead of creator command B for a
commandless child, including an explicit same-provider selection. Explicit child command C and a
different provider already worked. The fix preserves those overrides and inherits B only across
compatible native CLI routing within the same workspace and profile, with operator-home policy.

A live session retains its resolved command in memory. Resume with a stopped creator resolves the
existing lineage against current configuration. Deleted creator metadata retains the child's
configured fallback with an explicit `session.provider_route.creator_unavailable` log. No raw
commands or credentials are added to persisted metadata.

Startup failures now record the existing provider failure marker after canonical error/stop events.
Provider failure markers and session logs carry a SHA-256 command fingerprint. The selected-route
log has exactly one authoritative fingerprint, computed before adapter command rewriting. A
fingerprint identifies command configuration, not a provider account, login success, or profile.
Expected unsupported/missing ACP session loads retain their existing replay recovery markers,
without inserting a transient startup failure into the replay transcript.

## Executed evidence

- A fixed Go overlay disabling only the inheritance call reproduced two expected failures in
  `TestSpawnProviderCommandPrecedence`: actual `account-a`, expected `account-b`. The three explicit
  override/different-provider cases passed.
- `CGO_ENABLED=1 go test -race ./internal/session -run
  'TestSpawnProviderCommandPrecedence|TestPromptGenericFailureKeepsSessionActive|TestPromptRuntimeReplacementLifecycle'
  -count=1` passed.
- `CGO_ENABLED=1 go test -race -tags=integration ./internal/session -run
  'TestManagerIntegrationSpawnProviderCommandRouting|TestManagerIntegrationProviderPersistsAcrossCreateStatusListAndResume|TestManagerIntegrationFullStopResumeStopPersistsStopReasons'
  -count=1` passed in 11.414s. The existing integration suite launched real ACP helper subprocesses
  through account-labelled wrapper executables and used real SQLite. Files written by the executed
  wrappers proved B/B/C, no A launch for those sessions, and successful child resume with both live
  and stopped creators. Editing the creator definition after launch did not change its live route.
- `CGO_ENABLED=1 mise exec -- go test -race ./internal/session -run
  'TestSpawnProviderCommandPrecedence|TestSpawnProviderRouteDiagnostics|TestPromptGenericFailureKeepsSessionActive'
  -count=1` passed in 10.888s after the startup-marker fix. It checks isolated-home/environment
  boundaries, durable authentication-error recovery, startup failure retention, JSON log key
  uniqueness, matching route fingerprints, and absence of the private command value in logs.
- A broad uncapped `internal/session` race run had one one-second stop-deadline failure in
  `TestStopWithCauseLifecycle/Should_escalate_and_verify_exit_after_driver_stop_failure` under host
  contention. That unchanged case passed three isolated repetitions in 11.209s. No assertion or
  timeout was changed. The required scoped gate remains the delivery authority.

The final combined verification used `GOMAXPROCS=4 CGO_ENABLED=1 mise exec -- go test -race
-p=1 -parallel=4 -tags=integration ./internal/session` with the six focused tests listed above
(`TestSpawnProviderCommandPrecedence`, `TestSpawnProviderRouteDiagnostics`,
`TestPromptGenericFailureKeepsSessionActive`, and the three integration tests), `-count=1`.
It passed in 16.902s after the final routing-helper extraction and foreign-workspace case.
The targeted teardown was repeated afterward and remained clean.

The scoped gate exposed a replay regression from recording transient ACP load negotiation as a
provider transcript failure. Production marker emission was corrected; existing replay assertions
were unchanged. `GOMAXPROCS=4 CGO_ENABLED=1 mise exec -- go test -race -p=1 -parallel=4
./internal/session -run
'TestResumeReplayFallback|TestSpawnProviderRouteDiagnostics|TestPromptRuntimeReplacementLifecycle'
-count=1` then passed in 8.166s. This also verifies the final launch-time log and failed replacement
correlation: the failure identifies the attempted route while the prior runtime binding survives.

The test convention checker accepts the changed spawn suite. The other three existing test files
report the same pre-existing findings as their HEAD versions; the changed cases use parallel
`Should ...` subtests. Unrelated tests were preserved.

## Isolation and limits

The integration run used a uniquely allocated `issue-564-provider-routing` envelope, separate from
operator Compozy state, and each harness owned temporary directories and subprocess cleanup. The
run did not log in, log out, copy credentials, or change operator accounts. Targeted envelope
teardown is retained in `.cache/issue-564/teardown.json` with `clean: true`, no survivors,
and no processes requiring forced termination.

The account-labelled processes are controlled ACP fixtures. This proves command routing through
the real process boundary; it does not claim live two-account OAuth refresh or provider-service
availability. Prompt authentication failure is exercised at the unit I/O boundary. Web rendering
and unrelated wake/attention journeys were not repeated; the broader scenario's prior status is
unchanged. PR delivery additionally requires current-head CI and both bot review dispositions.
