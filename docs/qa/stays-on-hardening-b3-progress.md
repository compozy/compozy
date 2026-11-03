# Stays-on hardening B3 implementation record

## Contract and baseline

Worktree: `soh-b3-waits-context`; branch: `soh/b3-waits-context`.
Initial head: `a1b86dcae`; initial working tree clean.
Read the operator delivery plan, rules 20–26 and 30, Doctrine, Context,
Goal refusal, ADR-004 and ADR-007 in the workspace-root spec directory.

This is an implementation record, not full delivery evidence.

Turn 2 implements monitor-only idle release in the session supervisor. All monitor
kinds share `WorkSignalActiveMonitor`; scheduled waits belong to Loops and remain
blocking. Existing runtime-load and unknown-source guards remain intact.
The canonical `TestSupervisionReleasesIdleRuntime` now covers monitor release and
ACP reload, no-load retention, and mixed task-lease/scheduled-wait retention.
Bundled runtime guidance, configuration docs, and the monitor-wake QA scenario
are updated. The real-process IT-014 walk is still pending.

Two focused race runs passed before the final mixed-signal fixture adjustment;
final focused checks must be recorded before committing. The convention checker
reported seven pre-existing top-level-test findings outside the changed suite.
Installed lint was built with Go 1.26; the repository pins v2.15.0. Initial lint
failed on a shared lock, then produced stale line diagnostics during edits; rerun
on stable source before interpreting results.

## Owning paths and verification

| Contract | Implementation owner | Required cases |
| --- | --- | --- |
| Release restorable sessions holding only monitors | `internal/session/manager_idle_release.go` | UT-023, UT-024, IT-014 |
| Delivered monitor exclusion and dedupe | `internal/daemon/heartbeat_collectors.go`, `internal/store/globaldb/global_db_heartbeat_collectors.go` | Rule 22, retention arrival and departure |
| Lost monitor wake report | Session instance report path; coordinate ownership with B1 | UT-029 |
| Next wake and stale activity | Instance read projection; locate canonical suite before editing | UT-030, UT-031 |
| Instance and lineage doctrine | `internal/daemon/prompt_sections.go` | UT-032 |
| Usage reminder and compaction notice | `internal/situation/prompt_compaction.go`, `internal/session/query_context_usage.go` | UT-033–UT-036, IT-017 |
| Exhaustion-driven rotation | `internal/session/instance_report.go`, `internal/daemon/agent_instance_rotation.go`, global database wake state | UT-037, IT-018 |
| Goal refusal and active-Goal rotation gate | `internal/daemon/loop_goal_command_start.go`, `internal/store/globaldb/queries/instance_rotation.sql` | UT-049, UT-050, IT-022 |

## Shared maintenance dependency

`internal/session/contextusage/maintenance.go` currently returns `MaintenanceRotate`
at the threshold when compaction is unavailable. Its existing canonical suite is
`TestDecideMaintenance` in `contextusage_test.go`.

The helper has two consumers: instance rotation and Goal maintenance in
`internal/loop/goal/compaction.go`. The latter first calls it with `hasCompaction=false`
to decide whether to inspect available commands. Simply replacing the rotation
result with `MaintenanceNone` would silently disable that Goal maintenance path.
Account for both consumers when separating the reminder signal from rotation;
do not change unrelated Goal behavior incidentally.

The B3 invariant is that usage percentage alone never rotates an instance main.
Real exhaustion must persist `rotation_due_session`, and the existing admission
gate must remain authoritative, including the new active-Goal exclusion.
No compaction requests, input replay, transfer, handoff role, or fallback threshold
belong in the new instance behavior.

## Remaining work

Implement the paths above with existing canonical suites, then update bundled
Compozy references, site documentation, and affected QA scenarios. Generate schema
and wire artifacts through their generators. Run focused tests and lint only;
CI owns broad gates. Commit bounded changes locally, without pushing or opening
a PR. Report branch, final head, test mapping, checks, omissions, and cross-surface
impact to the orchestrator once implementation is complete.

Impact still pending: native instance reads, Goal set/replace, runtime prompts,
monitor and instance persistence, bundled skill documentation, Web next-wake
consumers, and public docs. Do not edit the central change-impact ledger.

Final turn-2 checks: `CGO_ENABLED=1 go test -race ./internal/session -run
'^TestSupervisionReleasesIdleRuntime$' -count=1` passed (7.762s); pinned
`golangci-lint@v2.15.0 run --allow-parallel-runners ./internal/session/...`
passed with zero issues. `git diff --check` passed. No full gates or E2E ran.

## Turn 3: monitor collector and timer deduplication

The daemon maps only undelivered/interrupted monitors into attention facts.
The public `monitors_fired` include name is retained. `TestMonitorDigestFact`
checks all four monitor kinds against the empty-history fingerprint through
active, delivering, fired, and canceled states; failures remain eligible.
The internal fact field is now `NeedsAttention`, with all consumers updated.

Timer retries match within the same session by description and deadline within
one second. The existing service suite verifies one ID and one wake. Fixtures
for capacity, cancellation, and observer coalescing now create distinct timers,
retaining their original assertions.

Checks: focused heartbeat/daemon collector tests passed with race (9.985s and
5.501s); the complete monitor package passed with race (1.458s). The test-shape
checker passed daemon and heartbeat suites. Its TestMain finding and four
pre-existing discarded-error findings in monitor helpers are outside this diff.
Integrated QA remains pending; documentation and the affected scenario updated.

Next implementation entry point: `instanceDefinitions.instancePayload` and
`projectInstanceMain` in `internal/daemon/agent_instance_reads.go`. Current
projection copies `LastActivityDetail` even when idle; UT-031 must assert it is
cleared unless activity is working. `AgentInstancePayload` currently has only
`NextWakeAt`, so `next_wake` requires additive wire/codegen work.

Final turn-3 lint: pinned v2.15.0 across heartbeat, daemon, and monitor passed
with zero issues. `git diff --check` passed. No broad gates or E2E ran.

## Turn 4: next wake and current activity

Additive `AgentInstanceNextWake` contains kind, description, and optional at.
The daemon reads only the current main's monitors after profile validation;
active/delivering monitors are eligible. Timer deadlines are known wake times;
terminal/session/pull-request watches do not invent a timestamp. Earliest known
timer wins over untimed candidates. Core payload composition then compares the
enabled active instance's next check. `next_wake_at` retains its existing meaning.
`now` is cleared unless the final projected activity is working.

UT-030: `TestAgentInstanceNextWake` and `TestInstanceMonitorReadModel` cover
ordering and untimed monitors. UT-031: `TestInstanceMainReadModel/Should clear
stale activity when the main is idle`. Focused race tests passed in core (3.458s)
and daemon (5.808s); pinned lint for core, contract and daemon passed, zero issues.
The test-shape checker passed the daemon suite; core's two old discarded-error
findings are outside the change. No integrated QA claim is made.

Initial codegen failed because installed openapi-ts lacked its repository patch.
`bun install --frozen-lockfile` repaired it; make codegen then passed. Bun 1.4.4
also changed the generated OpenDesign bundle, so rerun with pinned Bun 1.4.0
before committing. Go SDK split files shift after inserting the new DTO; no
hand edits to generated output.

Pinned Bun generation removed the unrelated bundle drift. `make codegen-check`
passed. SDK typecheck through root Turbo passed (2 tasks). Web adds only the
optional field via the generated operation response; visual presentation remains
F1-owned. Web typecheck/lint checks are pending before commit.

Final turn-4 validation: pinned codegen and codegen-check passed; root Turbo
SDK typecheck passed; root Turbo Web typecheck passed (4 tasks, including SDK
build). Web lint stopped at seven pre-existing format failures in proto/review
and src/drafts/stays-on-* files, none modified here. Because the lint lane did
not reach oxlint, this increment remains uncommitted pending scoped lint evidence.
Do not repair unrelated prototype formatting. The previous two commits remain
intact. No integrated QA, full gates, or E2E ran.

Turn 5 closes the focused Web check: root oxlint with deny-warnings and oxfmt
check both passed for the sole changed Web type file. This supplements the
passed Turbo typecheck; package lint still has the seven unrelated draft format
failures recorded above. The next-wake increment is ready for a local commit.

## Turn 5: doctrine

Next-wake increment committed as `99a4ee326`. Package Web lint still has unrelated
prototype formatting findings; focused changed-file oxlint/oxfmt passed, and
the commit hook also passed its staged format and lint checks.

Doctrine is a single daemon startup section gated by current instance identity
and user session type, or by an ancestor instance identity. Startup context now
carries those internal identities. Parent traversal reads session Status and
bounds cycles; missing historical parents end traversal. Runtime rebind resolves
its own persisted identity. The main-only Goal instruction stays out of children.
`TestStaysOnDoctrineStartup` uses the actual composed assembler for main,
descendant, ordinary session, and unbound conductor cases (UT-032).
Identity propagation and real-process QA still need stronger integrated coverage.
No Goal enforcement, reminder, notice, or exhaustion rotation is claimed yet.

Final doctrine focused race run passed: daemon 12.426s; session 10.172s.
Test-shape checker passed the changed prompt suite. Broader assembler-selection
regression and focused package lint results follow before commit.

After the lint-driven helper extraction, the final combined race selection
passed again: daemon 8.524s and session 8.073s. It includes composed assembler
and section selector suites. Next context entry point: situation/service_context.go
calls promptSections.compact at line 205; session.Manager.CompactionClearBoundary
returns the latest durable compaction boundary. No context changes made yet.

Final doctrine lint passed for daemon and session with zero issues.

## Post-compaction notice increment

Situation cache records the latest completed-compaction boundary and restores
full sections when it advances, with one notice on that prompt. The next prompt
can reuse stable sections again. A specific CompletedCompactionBoundary session
read excludes failed terminal observations, preserving existing telemetry's
broader CompactionBoundary behavior. Deps callback is wired at daemon boot.
UT-036 lives in TestAugmentCompactsRepeatedSituationSections. The existing
TestManagerEventsAndHistoryUseStoredEvents suite verifies that a failed later
terminal observation does not replace the successful completion boundary.
Checks run through native terminals with monitors; results pending. No commit
or IT-017 claim yet. Reminder and usage-drop reset remain to implement.

## Context reminder increment

ContextObservation reuses existing provider context telemetry and checks the
instance repository's current MainSessionID. Reminder at RotationThreshold is
one-shot until a usage drop or compaction boundary; unknown samples preserve
the last known occupancy. Any session's usage drop restores full situation
sections. UT-033/034/035 are added to the existing situation augmentation suite.
The real event suite passed (18.543s, race) for completed-only boundary reads.
Final tests and lint are registered on native monitors for terminals
term-fdc9a1ae0217 and term-f6dc48b140bf; do not commit before reviewing results.
Earlier lint read boot while the new callback file was being written and is
superseded by this final stable-source run. IT-017 remains pending. No rotation
or Goal-refusal changes are included in this increment.

## Goal refusal and active-Goal exclusion increment

The shared set/replace preparation checks the instance repository's current
MainSessionID. Other Goal controls retain their origin-validation path. Added
public reason `goal_on_instance_main`, enum registration, generated contracts,
and Web direction to use another session. InstanceRotationPersistenceReady
excludes nonhistorical session-origin live Goals, including paused/approval runs.
The canonical daemon Goal lifecycle suite adds UT-049/050: create a Goal before
binding a main, reject both set/replace afterward, allow status/pause/stop, and
check rotation readiness before and after Goal settlement using real SQLite.
Initial test fixture used the wrong agent name; corrected to its existing
operator-agent identity before the final monitored run.

Final Goal test is monitored on term-8a91ca842236. Codegen is monitored on
term-8963f685e5f9. No commit or completion claim yet. Context focused race tests
passed (situation 1.826s, session 11.146s). Lint line-length fix preserves exactly
the same prompt text. Remaining scope: exhaustion-only rotation and blocker
reports, real-process QA, final focused receipts and orchestrator report.

## Exhaustion-only rotation increment

Migration 00179 adds rotation_due_session with an empty default, preserving all
wake rows. ReportContextExhaustion inserts its idempotent failure report and marks
the current main in one write transaction. Descendant reports cannot mark a main.
Successful PublishRotation clears only the predecessor's marker in its transaction.
Reconcile eligibility now reads this marker and ignores percentages and advertised
compaction commands. Existing readiness gates and brief generation remain owners.
Blocked rotation reports immediately include concrete runtime or persisted blockers;
no compaction request, replay, transfer, handoff role, or fallback threshold added.
DecideMaintenance returns MaintenanceRemind instead of MaintenanceRotate; existing
Goal maintenance keeps its own distinct context_nudge_ratio behavior.

UT-037: TestInstanceRotationEligibility now asserts exhaustion-only eligibility,
unknown telemetry, provider compaction availability, and dependency blockers.
TestInstanceRotationDeferral verifies immediate named blockers and report dedupe
across reopen. TestInstanceRotationPublication verifies atomic exhaustion marking,
idempotent reporting, and marker clearing on publication. Migration integrity and
reopen suites and final package lint are monitored on term-bb27666e4c4e and
term-12d207fba07a. Final rotation/Goal tests: term-8c18098796d9. Latest codegen:
term-f8fe1b978968. Earlier codegen failures were sqlc CTE syntax/qualification,
repaired in the owning query; no generated files edited by hand.

Remaining before commits/delivery: review current monitor results, repair actual
failures, codegen-check, focused Web validation, real-process IT-014/017/018/022,
full case-to-test inventory, final scoped checks, commits and orchestrator report.

## Focused results and real-process QA progress

Final pre-helper-extraction rotation/Goal/contextusage race run passed: daemon
46.037s, globaldb 17.708s, contextusage 6.625s. Latest codegen completed with exit
0 (mon-5016614b1540), including migration 00179 and generated sqlc/client output.
Post-lint helper extraction checks are registered on term-339342583838 and
term-e1408f7572b1; final codegen-check on term-c35a68163771.

IT-017 now runs through the existing TestDaemonNativeCompaction with real ACP,
SQLite and HTTP. A test-local fixture uses typed CompactionNotice and SituationFull
match fields to require the first continuation's notice/full sections and a later
continuation without the notice. Shared fixture bytes remain unchanged. The
acpmock parser strips both new daemon wrappers before matching authored input;
its canonical stripping suite passed. IT-017 passed with race (85.103s).

IT-014 is TestMonitorIdleReleaseIntegration in the existing daemon integration
file: an active timer survives provider-process release and the next real prompt
loads the same ACP session. IT-022 extends TestInstanceNativeCreateJourney with
operator command ingress for set and replace on the actual instance main.
These real-process checks run on term-00a7b43e2249.

Migration suite's first parallel run hit its existing 180s deadline in historical
migration 00077, before the new migration. Re-run uses -p=1 -parallel=1 and
GOMAXPROCS=2, keeping assertions and deadlines unchanged. Do not claim migration
parity until that run completes. No commits yet for this combined pending work.

## Candidate case inventory

| Case | Canonical test |
| --- | --- |
| UT-023, UT-024 | session.TestSupervisionReleasesIdleRuntime monitor/no-load cases |
| UT-030 | core.TestAgentInstanceNextWake; daemon.TestInstanceMonitorReadModel |
| UT-031 | daemon.TestInstanceMainReadModel stale activity case |
| UT-032 | daemon.TestStaysOnDoctrineStartup |
| UT-033, UT-034, UT-035 | situation.TestAugmentCompactsRepeatedSituationSections reminder/rearm case |
| UT-036 | situation.TestAugmentCompactsRepeatedSituationSections completed-compaction case |
| UT-037 | daemon.TestInstanceRotationEligibility and TestInstanceRotationDeferral; globaldb.TestInstanceRotationPublication |
| UT-049, UT-050 | daemon.TestDaemonGoalCommandHandlerShouldExecuteCanonicalSessionLifecycle main refusal/control case |
| IT-014 | daemon.TestMonitorIdleReleaseIntegration |
| IT-017 | daemon.TestDaemonNativeCompaction (passed, 85.103s, race) |
| IT-018 | daemon.TestInstanceContextExhaustionIntegration |
| IT-022 | daemon.TestInstanceNativeCreateJourney Goal command checks |

Post-helper extraction focused race run passed: situation 1.879s, daemon 41.291s,
globaldb 20.944s, contextusage 1.711s. Codegen-check monitor reports exit 0.
Serial migration run passed internal/store (166.169s); globaldb reopen result
still pending. IT-014 initially ran while agent_progress was still fresh, so its
monitor-only precondition was false. Corrected the injected supervision time to
expire that evidence and explicitly assert only-monitor signals before the release
assertion. No production assertion weakened. IT-018 now scripts a real provider
refusal through ACP, verifies a named monitor blocker, then runs the actual
successor brief/publication/cleanup path. Final integration: term-58cdd6f59996.
Final lint after three remaining formatting/complexity findings: term-e869247c93d1.

## Final validation update

Pinned final lint passed with zero issues (term-e869247c93d1). Root Turbo Web
and SDK typechecks passed, 5 tasks, including final codegen-check (69s). IT-014
and IT-022 passed in the integration selection; only IT-018 failed because its
adopted test instance had an empty path ceiling. The fixture now permits only
the actual adopted workspace path, preserving the production ceiling check.
IT-018 rerun is monitored on term-f2161bc89df0. The context block is being
committed separately; no full B3 completion claim until all results and report.

## Forced-stop handoff (2026-11-03)

The orchestrator requested an end-of-turn stop and a clean committed branch.
The earlier remaining-work entries are chronological notes, superseded by this
handoff. All requested B3 production paths are implemented; final delivery is
not claimed while the verification limits below remain. No spec decision is
open. Rule 21 lost-wake reports belong to B1 in the operator delivery plan.

Committed increments before this handoff:
`9368ad8e2` idle release; `9bb593431` collector/dedupe; `99a4ee326` next wake;
`8c5e2ceef` doctrine; `f912bc4fd` reminder and completed-compaction notice.
The handoff commit includes exhaustion rotation, Goal refusal, migration 00179,
generated contracts, integration coverage, and documentation.

Final focused evidence:

- `term-f6b0b5cd5e64`: race integration selection passed (40.543s),
  TestMonitorIdleReleaseIntegration (actual timer wake reloads the same ACP ID),
  TestInstanceContextExhaustionIntegration (actual provider refusal, queued-input
  blocker report, cancellation and existing-brief successor publication), and
  TestInstanceNativeCreateJourney (set/replace refusal through command, actual
  HTTP route and native registry).
- `term-ba397d005052`: TestDaemonNativeCompaction passed with race (85.103s),
  requiring notice/full sections on the first continuation and no repeated notice.
- `term-339342583838`: focused race tests passed in situation (1.879s), daemon
  (41.291s), globaldb (20.944s), contextusage (1.711s), after helper extraction.
- `term-e869247c93d1`: pinned v2.15.0 lint passed, zero issues, for all affected
  default-build Go packages. `term-fdfc203e4cff`: added migration test lint passed.
- `term-adc2743e1ff3`: canonical globaldb reopen subcase for 178 to 179 passed
  with race (6.098s), preserving existing wake counters/schedule/digest and reports.
- `term-ae8007c0ec4a`: make codegen exit 0. `term-c35a68163771`: codegen-check
  exit 0. `term-7c1530c0bbde`: root Turbo Web and SDK typechecks, five successful
  tasks including codegen-check. Generators used pinned Bun 1.4.0.

Required unit case mapping remains in the inventory above; no required UT ID
is missing. IT-014, IT-017, IT-018 and the main-refusal part of IT-022 exist and
pass. IT-022 does not yet explicitly start a Goal on a newly created child in
the real-process journey; ordinary Goal creation is exercised by the canonical
Goal lifecycle suite, but that is not the same integration assertion.

Remaining validation and limits for the next agent:

- Integration-build lint (term-001e9a39201d) finished with 402 existing
  findings across the integration suite. Its diagnostics in subagent_integration_test.go
  are at unchanged lines 74, 299, 2523, 2528, 3194, 3355, 3396, 4861 and 5009;
  none intersects the added B3 test code. The first attempt hit the shared
  golangci lock; the completed run used --allow-parallel-runners and GOMAXPROCS=2.
  This is not a claim that integration-build package lint passes.
- The broader historical migration selection failed on its existing deadlines:
  an initial 180s context deadline during 00077; the serial retry passed
  internal/store (166.169s) but globaldb's whole historical reopen suite exceeded
  Go's default ten-minute package timeout. No deadline or assertion was weakened.
  The new migration's targeted preservation proof passed as recorded above.
- Full gates, full repository tests and E2E were not run, per the worker mandate.
  Integrated Web walks and F1 next-wake presentation remain with integration.
- Root Turbo typechecks pass. Package Web lint has seven existing unrelated
  draft formatting findings; the changed Web type/guidance files passed scoped
  lint/format checks.

Cross-surface impact: additive next_wake DTO on native/HTTP/UDS instance reads,
Goal reason vocabulary and shared command refusal, runtime doctrine/context
prompts, monitor collector/deduplication, additive global SQLite migration,
SDK generation, Web type/guidance, official skill and site docs, and the monitor
QA scenario. No tool IDs, hook APIs, authorization grants or config keys added.
No central change-impact ledger edit; the orchestrator owns that integration.
Migration 00179 may need renumbering if another slice uses the same version.
No push or PR was performed.
