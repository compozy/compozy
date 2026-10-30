# QA Run Report — 2026-10-29 — session-processes

- **Scope:** process discovery, ownership, explicit cleanup, keep across restart, and deleted-owner cleanup.
- **Cadence tier:** targeted. **Runtime verdict:** PASS.
- **Build:** feat/session-processes; runtime implementation in d8c9a7ca1, frontend through c984a2688; subsequent main integration is verified separately.
- **Environment:** isolated lab `session-processes-20261029-234903-302882`, backend port 62890, Web port 62891.
- **Walk:** 2026-10-30T00:08:01Z–00:27:29Z. Teardown reports `clean: true` with no survivors.
- **Parity:** acpmock/launcher authorized by the accepted brief; OS processes, daemon, SQLite, CLI and API are real. This is not live-LLM provider proof. Broad gates and E2E belong to CI.

## Personas and flows

Théo (Power User), desktop, local network, en-US. Journey `J-session-processes`, charter
`CH-session-processes`. The fixture supplies deterministic ACP behavior and launches ordinary
OS commands; discovery and termination use production code.

## Session matrix

| Leg | Result | Evidence under lab qa-artifacts/qa |
|---|---|---|
| Inspect and terminate | PASS: protected sleep attributed by lineage; Node by env; port 50619; stop with terminate removed both identities and closed the port | process-walk.jsonl |
| Keep and restart | PASS: sleep 7962 and Node 7967 retained the same start times after daemon restart; HTTP server on 50701 still returned 200 | server-before-restart.json, server-after-restart.json, process-walk.jsonl |
| Orphan cleanup | PASS: aggregate returned both retained identities; explicit termination returned terminated; both PIDs exited and port 50701 refused connections | kept-processes-terminated.json |
| Deleted owner | PASS: deleting sess-9f0384be435dbffb with keep retained sleep 3257 and Node 3270 in the aggregate; aggregate termination verified exit and closed port 50399 | deleted-owner-processes.json, deleted-owner-terminated.json |
| Native terminal shutdown | PASS: public CLI created term-aa2a4f73557c; OS identity captured PID 52240, parent 32369; graceful lab daemon stop removed that PID | terminal-shutdown-verified.json |
| Web | PASS for live chip, popover, port, lineage hint, and stop choices in both themes; no page errors reported by F | /tmp/session-processes-shots/final-live-chip-*.png, final-live-popover-*.png, final-live-stop-dialog-*.png |
| Teardown | PASS: manifest-scoped cleanup, no surviving lab processes, no operator runtime mutation | teardown.json |

Final live captures were repeated at 00:31Z using a fresh Vite process, session
`sess-d2d92a80c85dc178`, real Node port 58474 and protected sleep. Both themes show the
case-sensitive command display from c984a2688, without cache-busting or page errors. The dialog
was canceled; API stop with terminate then verified both PIDs exited and port 58474 closed.
A second exact-manifest teardown at 00:32Z reported clean. Its initial graceful stop did not
complete, so the helper used its identity-verified signal sweep; no survivors remained.

The live Web walk inspected the dialog and canceled it; API/CLI performed the destructive lifecycle
legs. Archive/delete and left-running visual states were captured in Storybook. Full Web end-to-end
execution, including `E2E-PROC-001`, is a CI gate and is not claimed as locally run.

## Focused engineering evidence

- `go test -race ./internal/procutil ./internal/sessionprocess ./internal/tools/builtin`: owning suites passed (separate final invocations after corrections). Exact identity, reused groups, startup protection, real env attribution and durable protected-binary lineage/reload are covered.
- Focused API/CLI/daemon checks passed, including policy body/query encoding and process ownership. The final UDS regex selected no tests; an earlier targeted UDS run passed. No claim of full UDS suite execution.
- Migration streams/equivalence and OpenAPI contract checks passed. Linux and Windows cross-builds passed; Windows is explicitly unsupported for process inspection.
- `make codegen-check` passed after regenerating the embedded linter with the pinned Bun 1.4.0 binary. No dependency update.
- Scoped Go lint reported zero issues. Test-shape checks found no added findings; existing findings in older suites were left untouched. The new lineage suite handles cleanup errors explicitly.
- The procutil niceness helper had competing cancellation and explicit cleanup owners. Its context now leaves process cleanup to the existing explicit owner; the full race suite passed without weakening assertions.
- F: root Turbo Web typecheck and focused Vitest passed (824 tests in the adapter-inclusive run), process controls/lib rerun 19/19, oxlint clean, React Doctor 100/100 before the final small adapter/display fixes.
- No local `make gate` or full E2E run: the accepted delivery brief assigns them to CI.

## Measured scan cost

Apple M4 Max, approximately 1,145 OS processes: tree scan 0.171 ms wall / 0.253 ms CPU;
environment scan 8.245 ms wall / 9.267 ms CPU. At 250 ms and 5 s respectively, these correspond
to approximately 0.101% and 0.185% of one core. These are scanner microbenchmarks, excluding
port/CWD enrichment and the service owner lookup; they are not total daemon CPU measurements.

## Findings and limitations

macOS protected binaries that detach and change PGID before the first observation cannot be
attributed. Daemon shutdown preserves detached descendants and closes native terminals; PTY
restart survival is out of scope. Legacy unknown markers cannot establish profile ownership.

Review repaired empty-workspace User-scope queries and aggregate batches above 100 targets.
Command display uses case-sensitive path matching for Linux and case-sensitive APFS safety.

F observed an existing light-theme destructive Button contrast issue in Storybook. No tokens were
changed. Only the final-live captures are final UI evidence; earlier captures are retained as intermediate artifacts.

## Final status

PASS for the bounded runtime walk. The strict evidence audit passed with zero blockers, and manifest-scoped teardown
reported clean with no survivors. Both artifacts are recorded with the lab. PR readiness remains contingent on current-head CI and review completion.

## Published live evidence

- [final-live-chip-dark.png](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/session-processes/d8c9a7ca1/01-final-live-chip-dark.png)
- [final-live-chip-light.png](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/session-processes/d8c9a7ca1/02-final-live-chip-light.png)
- [final-live-popover-dark.png](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/session-processes/d8c9a7ca1/03-final-live-popover-dark.png)
- [final-live-popover-light.png](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/session-processes/d8c9a7ca1/04-final-live-popover-light.png)
- [final-live-stop-dialog-dark.png](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/session-processes/d8c9a7ca1/05-final-live-stop-dialog-dark.png)
- [final-live-stop-dialog-light.png](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/session-processes/d8c9a7ca1/06-final-live-stop-dialog-light.png)

## Integration with main

Merged main e6457afe0 (#1047 project folders, #1048 spawn budgets). Its migration 00156 is
preserved byte-for-byte; the canonical generator emits process observations as 00157. Regenerated
SDKs, `make codegen-check`, race tests for sessionprocess/builtin/spec, migration equivalence and
focused HTTP/daemon wiring checks passed. The merged Web typecheck passed (4/4 Turbo tasks).
F ran 2,182 passing tests across 160 files; one additional palette suite failed during import because
its fixture path resolves outside this worktree. That known path issue is not a passing test and
remains subject to CI's normal checkout. The stream merge preserves both workspace-layout refresh
and process-query invalidation.

## Review and CI remediation

Greptile identified stale process rows when identities changed without changing totals/ports.
Catalog invalidation now compares identities and displayed process data; owning regressions cover
replacement, PID reuse, command/CWD changes, owner changes and unchanged observations. Process
stop controls now serialize requests so one pending identity cannot be hidden by a second stop.
The reported composite-literal compilation issue was a Go-version false positive: the pinned
Go 1.27.1 compiles these literals and executed the domain tests on both macOS and Linux CI.

CI also exposed missing native visual metadata and transport inventory entries, two oversized
frontend files, and formatter drift in the generated emoji mirror. Production metadata and exact
contract expectations were completed, frontend modules extracted, and the mirror formatted with
its existing formatter owner (semantic generator check passes). Focused HTTP/UDS/toolmeta race
checks passed. Sessionprocess race tests and lint passed; Linux procutil lint and cross-build passed.
Current-head CI remains the final delivery gate.

A subsequent CI pass found the new CLI command missing from authored navigation and the process
observation table missing from the User-scope migration inventory. Both production inventories
were completed without changing their existing test assertions. The navigation test and User-scope
migration race suite passed. API route registration and catalog event streaming were split at
their existing responsibilities to satisfy function limits; targeted API/CLI race tests passed,
and Linux lint over core, HTTP, UDS, API spec and CLI reported zero issues.

Runtime CI caught a launch-environment regression: the new process marker overwrote
`COMPOZY_PROFILE` with an ID, while the CLI expects a profile name. Attribution now uses
`COMPOZY_PROFILE_ID`. Existing launch and real-process suites demonstrated the failure before
the fix and passed afterward with the race detector. The full existing fallback-ledger and Goal
command E2E functions passed in a targeted local run (32.904 s); the Goal stop/removal case also
passed three repetitions (17.370 s). Its earlier CI pause failure did not reproduce, and no Goal
assertion, timeout or production behavior was changed. Linux session/procutil lint reported zero issues.

All four Web E2E shards and desktop E2E passed on CI head `5c51a2f33`. These results precede the
final environment correction; required checks on the eventual PR head remain the delivery gate.

## CodeRabbit remediation evidence

The canonical suites reproduce and cover keep during inspection failure, rejection of
unsupported lifecycle cleanup before mutation, cleanup after a disconnected delete or waited
stop, CLI process-policy forwarding and unsupported exit status, audit failure across multiple
targets, startup inspection recovery, and summary-port/count consistency. No standalone test
file was added. The real-process suite verifies a process joining an inspected group is not
signaled, while existing detached env and durable lineage/restart cases still pass.

Focused race results: CLI 4.951 s, sessionprocess 5.197 s, procutil 1.862 s, native descriptors
1.670 s, and HTTP handlers 2.099 s (after correcting the new fixture's explicit session lookup).
Code generation and the OpenAPI owning suite passed (4.346 s); Windows cross-build passed.
The native selector schema retains the provider-compatible top-level object contract. Shared
handler validation rejects ambiguous selectors before signaling; the handler suite checks the
forwarded selection as well as rejection behavior.

The final HTTP race rerun passed (2.053 s), scoped Linux lint reported zero issues, and
`make codegen-check` passed. The existing Goal runtime E2E function also passed locally
(31.076 s). CI on a19d5c416 nevertheless reproduced the Goal pause/run-status mismatch;
that local pass did not explain the Linux failure. The confirmed cause and subsequent Linux
verification are recorded below under Goal stop ordering regression.

The follow-up ownership regression failed against the original observation upsert: a second
save changed the in-memory owner but a durable read retained the old session, workspace and
profile. The upsert now persists all three owner fields. The existing globaldb process-record
suite passed with the race detector (3.246 s), scoped Linux lint reported zero issues, and
code generation plus `make codegen-check` passed.

## Goal stop ordering regression

The repeated CI failure was reproduced in an isolated Linux runtime with the existing Goal
command E2E function. With one CPU assigned, two of ten repetitions left Stop in
`needs-approval` with `goal_recovery_ambiguous`. The interrupted prompt setup observed
`stopping` before Goal pause persistence completed, failed with `session is not active`, and
committed ambiguity first. A diagnostic-only run identified that original setup error.

The Manager now serializes this setup recovery against the stop state/Goal-pause transition,
and releases the lock before waiting for setup to drain. A regression in the existing stop
lifecycle suite failed before the fix and passes afterward. Existing assertions and timeouts
are unchanged. Owning stop/managed-input race suites passed (8.726 s), scoped Linux lint
reported zero issues, and the unchanged real Goal E2E function passed ten Linux repetitions
with one CPU plus five Linux repetitions with the race-enabled test binary and two CPUs.
The daemon and mock binaries used Go 1.27.1; the test containers used isolated temporary
runtime state and removed themselves on exit. No operator daemon was touched.

## Final Web delivery coverage

The existing process E2E suite now includes `E2E-PROC-002` for session-list archive and delete,
with both process choices. It drives the real dialogs, verifies successful policy-bearing
requests and independently observes process survival/exit; execution remains assigned to CI.
The aggregate adapter intentionally serializes destructive batches. A narrow React Doctor
exemption records that resource constraint, following existing repository practice. The warning
reproduced without it (93/100); the pinned changed-scope scan passed with it (100/100), while
the adapter regression still verifies one request in flight and per-target failure reporting.
The final root Turbo Web typecheck also passed all four tasks after both frontend commits.

The later source-size check found the session registry at 504 lines after documenting cleanup
errors. The Stop operation definition was moved unchanged into its own registry file, matching
the neighboring session-operation files. The source-size check, API-spec race suite (28.401 s),
and codegen-check passed; the generated public contract did not change.

## Native process policy parity

The native registry regressions reproduced scan failures blocking omitted/keep policies, internal
scan errors classified as invalid input, unsupported termination mutating the session, and caller
cancellation leaving accepted cleanup unfinished. Native stop/archive now retain keep behavior,
reject unsupported termination before mutation, and classify operational errors separately from
invalid policy. Accepted waited termination and cleanup use bounded independent contexts. The
registry still reports cancellation to the caller; the cleanup outcome is checked independently.
The existing native suite also asserts workspace/profile fencing at the process service boundary.
The focused native policy, archive-refusal, and session-orchestration race suites passed (5.163 s).
Scoped Linux daemon lint reported zero issues, and the production source-size check passed.

## Final review scope hardening

Aggregate cleanup now restricts targets to retained owners in the selected mutation profile.
The existing HTTP handler suite checks live/foreign refusal and same-profile cross-workspace
eligibility; focused HTTP and native daemon race suites passed (2.020 s and 4.545 s).
The query-cache regression reproduced missed invalidation after adding profile identity; prefix
invalidation repairs it and all 24 query-options tests pass through root Turborepo.
Bun 1.4.4 produced an embedded-linter mismatch; the pinned Bun 1.4.0 passes codegen-check.
The terminal bridge returns success when graceful-close fallback finds the original identity gone
or replaced, preserving refusal to signal replacement identities. Focused Go race suites pass for HTTP, native policy, sessionprocess and procutil.
The omitted-wait native stop regression now propagates caller cancellation from its I/O boundary;
the bounded independent legacy stop passes (5.207 s). Root Turbo checks pass for 116 process
adapter/control/cache tests and 129 adapter/cache/stream tests; lint/typecheck pass with zero
lint issues. Scoped Linux Go lint reports zero issues and Windows daemon cross-build passes.
React Doctor returns 100/100 after keeping the existing sequential-batch exemption adjacent
to its await. Exact-head CI and both external reviewers remain required.
The test-shape checker reports pre-existing findings outside the changed HTTP subtest.

The aggregate Web action carries each target's profile and splits requests at profile boundaries.
Catalog events use the existing five-second aggregate reconciler, while session process prefixes
remain promptly invalidated; reconnect recognizes the profile-suffixed process keys.
