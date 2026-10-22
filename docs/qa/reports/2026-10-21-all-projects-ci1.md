# All Projects CI1 — Go regression repair

Scope: the failing Go packages from CI run 39710841905, on branch `ap/ci1`.
The dispatch explicitly permits package tests and prohibits `make gate`, repository-wide tests,
pushes, and PR creation. No migration or generated client contract changes are required.

## Root causes and fixes

| Owner | Root cause | Repair and owning evidence |
| --- | --- | --- |
| Task authorization | `UserProfileID` was compared with every task's profile, including project actors and extension/automation actors. Project tasks were denied without a policy; User actors were also denied their own User tasks. | Recognize same-scope User and project actors; always consult the injected session policy for agent sessions so authenticated cross-profile decisions remain authoritative. `internal/task` and Host API suites. USW BR-6, BR-8, BR-10; T-22/T-24/T-34. |
| API core | Helper fixtures lacked an HTTP request after origin propagation began using request context. User identities also bypassed the project requirement for extension dev/reload. | Supply real HTTP requests to helper fixtures; enforce the project guard before extension dispatch. Existing transport and extension handler suites. USW BR-9. |
| Native profiles | Generic profile selection interpreted task execution-profile objects as profile names and consumed extension installation selectors. | Apply the generic selector only to workspace-bearing descriptors; preserve extension lifecycle ownership of installation profiles. Existing native task profile and scoped extension update tests. USW T-34. |
| Native skills | The new implicit `@user` selector entered project resolution. | Parse the User reference before resolving profile-only skill configuration. Existing authored/package agent projection test. USW BR-5. |
| Startup manuals | The combined bundled references exceeded the 80,000-rune cap. | Keep trusted bundled manuals complete when skills are disabled and no on-demand read exists. Existing composed assembler contract. |
| CLI sessions | Aggregate JSON/JSONL and empty pages unnecessarily fetched project display names. | Fetch labels only for populated human/TOON output. Existing profile JSONL and aggregate badge tests. AP BR-10 and public-output compatibility. |
| Daemon fixtures | Scheduler fixture registered COMPOZY_HOME; task role expectation omitted the deferred creation origin. | Use a separate project directory and assert the automation origin. USW BR-3/BR-9; T-05/T-17/T-37. |
| Heartbeat | A missing workspace is now valid User work, so it no longer supplies the invalid member of a batch. | Use a missing agent name for the invalid request; preserve ordered success/failure assertions. USW User heartbeat acceptance. |
| Settings/OpenAPI/routes | Expectations predated grouping translation, optional PATCH fields, User aliases, owner fields, and five aggregate routes. | Update exact expected contracts and the compressed historical schema fixture for approved USW/AP additions only; keep emitted global/workspace enums unchanged. AP BR-9/BR-14, ADR-004; USW BR-17. |
| Global database fixtures | Current session writer ran against a historical schema; retirement compared `SELECT *` across added columns; empty workspace validation predated profile-keyed User config. | Seed the historical session shape with its profile, compare every original column across upgrade, and assert the missing User profile error. Real SQLite migration suites. USW BR-8/BR-15; T-12/T-18. |

## Test invariants and placement

All changes reuse existing suites. No standalone test file was added. Fixtures preserve task audit,
lease fencing, blocking, idempotency, creator wake, migration data preservation, and transport
invariants. Global-task agent fixtures now represent same-profile User sessions (USW BR-6/BR-17);
project-specific cases retain project actors. The task authorization suite additionally checks that
same-project cross-profile decisions still reach the injected policy and honor denial/grant.

The schema fixture is an executable public-artifact contract. Its exact comparisons remain intact;
only the approved USW/AP shape is incorporated. Production definitions and generated outputs are unchanged.

## Verification

Race invocation: `CGO_ENABLED=1 go test -race -gcflags=modernc.org/...=-d=checkptr=0 <packages> -count=1`.

- Complete packages passed: `internal/task`, `internal/api/core`, `internal/api/spec`,
  `internal/heartbeat`, `internal/settings`, `internal/cli`, `internal/api/httpapi`, `internal/api/udsapi`.
- `internal/daemon`: the complete package exposed four remaining failures; after repair, all affected
  groups passed: `TestDaemonNativeTools`, `TestDaemonExtensionInputLifecycle`,
  `TestComposedAssemblerAssembleStartupLoadsBundledToolsSectionDescriptor`,
  `TestNativeSkillsForSessionAgentUsesLiveAuthoredDefinition`,
  `TestBootTasksSchedulerStatusUsesDurableStarvationEpisodes`, and
  `TestSessionPolicyGateBuildsConcreteTaskRoleCreateOpts`.
- `internal/store/globaldb`: `TestGlobalDBUserFolderMigration`,
  `TestGlobalDBMemoryTriggerRetirementMigration`, and `TestGlobalDBLoopConfigShouldPersistOverrides` passed.
- `internal/extension`: the package run passed the repaired Host API cases but hit a timeout in
  `TestManagerDisablesExtensionAfterConsecutiveFailures` while other suites ran. The focused race
  rerun of `TestHostAPI*` and that exact timeout case passed; no timeout assertion was changed.
- Test-shape heuristic: zero new findings compared with HEAD. Existing findings in the large canonical
  suites remain untouched.

## Impact and limits

The owning impact audit remains the USW/AP spec audit; the coordinator consolidates it.
Native tools, extension Host API, profile/workspace isolation, CLI output, and startup guidance are
covered above. No new hooks, config keys, public DTOs, schemas, migrations, or official skill text.
Web/Docs feature behavior is unchanged; this slice restores the approved contracts.

The adjacent QA scenarios are `ET-native-workspace-scope-isolation` and
`ET-profile-scoped-work-reads`. This report is package/integration evidence, not a fresh hosted-agent
or browser walkthrough. The orchestrator retains the final cross-surface QA and current-head CI gates.

## Delivery blocker

The code and focused race evidence are ready for integration, but no commit was created.
`git add` cannot create the shared worktree Git index lock outside the writable sandbox.
Automatic approval review also failed before executing the additional `go vet` command because
its access token could not be refreshed after logout/account switching. This is an approval-service
authentication failure, not a finding that the commands are unsafe. Re-authenticate the approval
service before retrying Git staging/commit; do not bypass that review.

The pre-commit hook runs lint-staged, whose default backup uses `git stash`. The session explicitly
forbids stash. Once authorization is restored, run the equivalent lint-staged check with
`--no-stash --no-hide-partially-staged` and prevent the default hook from creating a backup stash.
Do not run `make gate` for this delegated slice, and do not stage `extensions/open-design/lint.gen.mjs`.

Scoped golangci-lint v2.13.1 reported two `golines` findings and then exceeded its five-minute
analysis timeout. Both formatting findings were repaired with that exact formatter binary;
unrelated formatting produced by that formatter was removed from the diff. The complete scoped lint is **not green**.
`git diff --check` is clean. All final race receipts above precede formatting-only changes.

Intended commit: `fix: repair Go CI regressions across User and project scopes`.
Its body should cite USW BR-3/BR-6/BR-8/BR-9/BR-17 and AP BR-9/BR-14/ADR-004 for the deliberately
changed fixture/schema expectations. No codegen ran; no migration files changed.
