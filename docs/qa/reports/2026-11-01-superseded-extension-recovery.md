# Superseded extension upgrade and recovery

Target: bundled upgrade and interrupted-install recovery, issue #1105.

The isolated seeded-home integration `TestBootContinuesAfterCorruptExtensionAndKeepsHealthyExtensions`
passed with `CGO_ENABLED=1 go test -race -tags=integration ./internal/daemon -run
'^TestBootContinuesAfterCorruptExtensionAndKeepsHealthyExtensions$' -count=1`.
It seeds a disabled bundled `dev-cycle` through the production registry, boots the daemon against
that home, verifies retirement and preserved successor enablement, and retains healthy extension
hooks despite an unrelated corrupt package. The fixture owns temporary homes and sockets and
shuts down its daemon; it does not restart the operator daemon. Its existing session, observer,
and server fixtures remain test doubles, so this is boot/runtime integration evidence rather
than a live CLI or browser walk.

Focused race-enabled suites passed for:

- `TestRegistrySupersededBundledMigration` and `TestRegistrySupersededBundledOwnership`: exact profile
  attachment transfer, preserved disabled state, input retention, conflicting effective enablement,
  user-owned name protection, and idempotency.
- `TestReconcileManagedExtensionArtifacts`: a later startup retains retired package files while
  ordinary unregistered residues still clean up.
- `TestManagerStartContinuesAfterParseFailure`: a healthy target reload succeeds while the unrelated
  failure stays visible in status.
- `TestDaemonExtensionServiceRollsBackFailedInstallReload`: a real manager and SQLite registry install
  a healthy resource-only package alongside a broken provider, remove an orphaned package through
  the operator service, reject an agent's orphan removal, and compensate a genuinely invalid package.
- `TestDoctorExtensionFilterReportsDistinctRuntimeFailures`: the diagnostic names the superseded
  bundle and gives the exact global remove command.
- Existing registry installation-scope, manager reload, lifecycle-coordinator, and spec-cycle managed
  install suites also passed in the focused runs.

An initial integration run inherited a daemon restart-operation selector from the managed terminal
and refused the unrelated operation ID in the isolated database. The scenario now clears that
selector with `t.Setenv`; the rerun passed. The earlier compile failure from an incorrect severity
constant was corrected before successful focused runs.

No local gate, full test/lint suite, E2E suite, browser walk, or Windows runtime validation was run.
CI owns the broad gates and final-head verification. The changed code is Go and documentation;
React Doctor applicability must be confirmed from the PR check evidence.

The test-convention heuristic passed the doctor file. It reports pre-existing subtest naming,
inline-case and discarded-error findings in the large registry, manager, daemon and managed-install
files; touched cases conform, and unrelated suites were preserved. Changed Go files were formatted
with the repository's configured formatters without running a lint lane.

The current base already deletes a failed install's registry row before its compensating reload.
The historical orphan reported on 0.4.7 was therefore seeded explicitly for recovery verification;
this work does not claim the 0.4.8 base recreated that exact half-write.
