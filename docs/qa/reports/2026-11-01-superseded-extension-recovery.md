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

## Review remediation

Greptile review `5730482462` identified three valid defects on the first candidate. Successor
enrollment and state transfer now share the registry installation transaction; the regression
aborts predecessor retirement and verifies no half-enrolled successor exists before a lossless
retry. Marketplace reload callbacks bind the current candidate name for both commit and rollback.
Orphan removal enumerates persisted credential cells across profiles/workspaces and retires their
owned references plus MCP reservations through compensating removal.

The focused `TestExtensionSecrets/Should_retire_orphan...` integration seeds global and two workspace
credential cells, forces MCP retirement to fail, checks credential restoration, then removes and
reinstalls the package without inheriting credentials. It passed with `-race`. The existing global
and development retirement case and seeded-home boot case passed in the same focused integration run.

CodeRabbit review `5730521177` added scoped Doctor commands, shutdown error isolation, and stronger
coverage. Registry-owned diagnostics now include workspace-only and unattached registrations;
commands select their actual attachment and profile. Source ownership is tested in the daemon's
canonical diagnostics suite, while the API suite verifies transport composition. Selected reload
coverage checks both relaunch PID and isolation of unrelated shutdown errors. Existing-successor
scope coverage checks preserved shared timestamps and transfer of a distinct attachment.

The first-head lint job reported function length, long lines, missing SQL contexts, and one extra
blank line. These were repaired with smaller cohesive helpers, context-aware fixture SQL, and
changed-file formatting. Lifecycle and migration docstrings describe the state-preservation
boundaries requested by the review warning. No full local lint lane was run.

Greptile's second review (`5730585731`, comment `4437635571`) found that the CLI's global
removal branch dropped the selected profile. Removal now reuses the existing extension profile
selection boundary before choosing either transport. `TestExtensionRemovalProfileSelection`
passed with `-race` for a root flag, environment selection, flag precedence, and a typed unknown
profile refusal before any removal. The existing scoped-read and extension client transport
suites passed alongside it. No flags, DTOs or generated contract sources changed.

The second-head lint run reported a single large range-value copy in the new diagnostics owner;
iteration now uses indexed references. Broad verification remains in CI.
The existing `TestExtensionCommandRoundTripIntegration` also passed with `-race` after the CLI
profile fix (36.7 seconds). It exercises the live CLI transport and extension lifecycle fixture;
its existing integration-service boundaries remain as described by that canonical suite.

The final CodeRabbit remediation keeps migration SQL in generated sqlc queries behind a globaldb
repository mapping, sharing the existing enrollment transaction. Focused registry race regressions
verify the unchanged atomic transfer and refusal behavior. The batch test now requires the actual
successful candidate reload, and Doctor coverage checks both global and workspace items together.

Orphan credential integration additionally seeds Vault writes with no binding row in global and
workspace namespaces. A forced MCP-retirement failure restores them; successful removal and
reinstall leave none behind. A neighboring extension's secret and an operator-managed secret
under the same owner prefix remain intact. The sweep holds the exclusive package mutation lock,
validates exact encoded ownership, checks remaining binding references and snapshots compensation.

Runtime-health regressions cover a server resolved before package removal whose exchange begins
only afterwards, including reuse of the same bundle checksum. Resolution and exchange observations
carry lifecycle fences; full package retirement clears every workspace without clearing another
package's diagnostics. Instance-only detach continues to evict only that workspace.

The remediation reran the seeded-home boot integration after moving migration statements to sqlc;
it passed. `make codegen` and `make codegen-check` passed with the locked Bun dependencies and
explicit pinned tool binaries. An initial generator check exposed missing/patched dependency and
tool-shim version drift; no unrelated generated changes remain. Focused MCP resolver, registry,
Doctor and runtime-health race checks are the local verification boundary. CI owns broad gates.

The next review pass added an owner-filtered sqlc metadata query, so orphan recovery does not
materialize other packages' secret records. The existing ref index bounds the global-owner and
workspace namespaces; the workspace encoding still requires filtering within its namespace and
no schema/index migration is introduced. Focused globaldb tests cover exact encoded owners,
multiple workspaces and neighboring prefixes.

Generic HTTP/UDS Vault PUT/DELETE retain their public ref contract and share the existing package
mutation lock for extension-owned refs. A focused integration exercises the real core HTTP
handlers, SQLite Vault and lifecycle coordinator: both mutations wait until restoration completes,
then their newer write or deletion wins. It uses the existing isolated fixture, not a live server
or an authorization-boundary end-to-end journey. The orphan cleanup integration was rerun and passed.

The final test refinement builds the production `httpServerOptions` and `udsServerOptions` and
registers their real routes in isolated Gin engines. Both PUT and DELETE are exercised through
each option set while the package lock is held; dropping either production wrapper makes the
scenario fail. No TCP or UDS listener is opened. Existing minimum session/task/observer/workspace
fixtures supply unrelated server dependencies. The package-health regression additionally proves
a second-workspace failure exists before retirement and rejects a late resolved call there.

The final Vault scenario calls the real operator `Remove` entry point on an installed package
seeded into the historical orphan state. A barrier at the Vault metadata I/O boundary pauses
retirement while generic HTTP/UDS mutations queue. A real SQLite trigger then fails MCP retirement,
forcing production registry/credential compensation. The scenario requires the injected error,
restored package registration, and the later PUT/DELETE outcome. This replaces modeled restoration
with the production removal path; the focused race-enabled integration passed for all four cases.
The MCP registry suite also passed with both stale workspace keys replayed after fresh same-bundle
resolution, preserving only the new installation's outcome.
