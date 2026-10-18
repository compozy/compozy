# QA Run Report — 2026-09-28 — session-continue-fork (part B1: real-agent CLI/API walks)

- **Scope:** branch `continue-fork` — session continue/fork (derive primitive, native fork, retry receipts, `handoff` provider-error offer, provenance rewind) and the fallback-account prerequisite (agent `fallback_chain` seats, role fallback route `command`). Part B1 of task_08; B2 owns the web walks and VC bundles, C owns the pre-feature upgrade walk (`RT-session-lineage-upgrade`).
- **Cadence tier:** targeted
- **Build:** `79c7d4b57` + uncommitted task_08 changes (part A, B2's in-flight web/config edits, and the B1 fixes below), rebuilt into the lab after each fix · **Environment:** isolated lab `compozy-session-continue-fork-b1-20260929-010817-219689-lab` (own `COMPOZY_HOME`, UDS, HTTP 127.0.0.1:64133), real `opencode` 1.18.33 (`opencode acp`), `claude` 2.1.280 via claude-agent-acp (`claude-haiku-4-5-20251001`), `codex` 0.157.1 via codex-acp 2.0.0 (`gpt-5.6-luna`), acpmock driver built from the branch. Parity notes: `roles.auto_title` and `roles.memory_extractor` disabled except for the MS walk (to keep real-provider cost at a few one-line turns); lab-local acpmock fixtures under the lab (`long_turn_fixture.json`, `retry_target_fixture.json`, `seat_rate_fixture.json`).
- **Plan:** `docs/qa/reports/2026-09-28-session-continue-fork-plan.md`
- **Evidence:** `docs/qa/evidence/2026-09-28-session-continue-fork-b1/` (gitignored, lean copies); raw lab scratch under the lab `qa-artifacts/ev/`.
- **Started:** 2026-09-29T01:08Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Rafa | operator on the CLI/API | desktop / wifi-fast / en-US | CH-session-derive-real-agents, CH-session-fallback-seat |
| Théo | interrupt-minded operator | desktop / wifi-fast / en-US | CH-session-derive-interrupt (minus the upgrade leg) |
| Ada | platform admin | desktop / wifi-fast / en-US | CH-role-fallback-boundary |

## Flows in Scope

- `J-15-operate-session-via-cli-api` — continue/fork/retry from the CLI and HTTP (`../journeys/J-15-operate-session-via-cli-api.md`)
- `J-route-background-work` — seat and role fallback before acceptance (`../journeys/J-route-background-work.md`)
- `J-rewind-conversation` — provenance-child rewind leg (`../journeys/J-rewind-conversation.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-derive-real-agents | J-15 / ET-cli-session-continue | Rafa | Money | Pass | | |
| 2 | CH-session-derive-real-agents | J-15 / RT-session-derive-native-fork | Rafa | Money | Fixed | BUG-20260928-native-fork-clone-load-refused | uncommitted |
| 3 | CH-session-derive-interrupt | J-15 / RT-session-derive-retry | Théo | Interrupt | Fixed | BUG-20260928-derive-replay-deleted-child-origin-lost | uncommitted |
| 4 | CH-session-derive-interrupt | J-15 / RT-provider-error-handoff (CLI/API leg) | Théo | Interrupt | Fixed | BUG-20260928-handoff-stream-error-says-retry | uncommitted |
| 5 | CH-session-derive-interrupt | J-rewind-conversation / RT-conversation-rewind (provenance leg) | Théo | Interrupt | Pass | | |
| 6 | CH-session-fallback-seat | J-route-background-work / RT-session-fallback-chain | Rafa | Network | Fixed | BUG-20260928-route-command-env-prefix-not-launched | uncommitted |
| 7 | CH-role-fallback-boundary | J-route-background-work / MS-background-role-fallback | Ada | Network | Fixed | BUG-20260928-route-command-env-prefix-not-launched | uncommitted |
| — | (controller request) | RT-session-lineage-upgrade step 6 | Théo | Interrupt | Fixed | BUG-20260928-derive-stopped-source-turn-in-progress | uncommitted |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

Row 5 covers only the branch-created provenance leg; the scenario's overall verdict waits on B2's web leg. The "Fixed" rows were re-walked on a daemon rebuilt with the fix.

## Session Debriefs

### CH-session-derive-real-agents — Rafa

- **Ran:** 01:10Z → 01:24Z (box respected: yes)
- **Findings:**
  - Continue Codex → Claude works end to end: Golden Path block, Claude's own session log shows `Context rebuilt from log.` + `{compozy_context_replay}` (4 Codex messages) + `User request:`, the child answered the codeword, source fences/meta/runtime unchanged, one `session.derived`, CLI JSON = HTTP JSON field set, usage errors exit 2.
  - All three real adapters (OpenCode, claude-agent-acp, codex-acp) now advertise `session/fork` + `session/load` + `session/resume`. OpenCode and Claude native forks load and answer from the clone; Codex native fork left the child permanently pending while the source lived (Blocks-Completion) → fixed.
  - Scenario step 5 ("a Claude source returns seed replay") is stale for today's adapters; the replay path was walked on a message cut, a running source, and a stopped source instead.
- **Bugs filed/updated:** BUG-20260928-native-fork-clone-load-refused (new, fixed), BUG-20260928-claude-inspection-close-timeout (new, fixed; pre-existing on main, surfaced here).
- **Scenarios settled:** ET-cli-session-continue → pass; RT-session-derive-native-fork → pass (fixed).
- **Paper cuts:** unknown agent human output lacks the `agent_not_found` code; `compozy logs --session {id}` needs a registered cwd or `--workspace`.
- **Surprises:** claude-agent-acp prefixes replies with "**Auto mode unavailable:** … using Accept edits instead." (adapter notice, not Compozy text).
- **Suggested next charter:** native fork on a Claude/Codex source running a non-default account route (command fingerprint gate) with real binaries.

### CH-session-derive-interrupt — Théo

- **Ran:** 01:24Z → 01:38Z (box respected: yes)
- **Findings:**
  - Retry receipts behave as specified (same key → `Replayed yes` with recorded counts, conflict 409, stale fences 409, deleted child → `Child deleted yes`, nothing created). The deleted-child replay printed `--` for the origin agent → fixed.
  - `--message` returns only after the first bind dispatched the message (6 s for a slow-start agent), so the "admitted, not dispatched" crash window is not reachable from public surfaces; a `kill -9` right after the return interrupted the in-flight turn, and the retry did not re-run it (one `user_message`, one `session/prompt`).
  - Handoff: the persisted event and marker prescribe `handoff`, but the CLI stream said `next_action=retry` → fixed. Spawned child (real Claude spawning `handoff-agent` via `compozy__session_spawn`) keeps `retry`/`inspect`.
  - Provenance rewind (branch-created `--parent` child) rewinds in place; spawned child → 409.
- **Bugs filed/updated:** BUG-20260928-derive-replay-deleted-child-origin-lost, BUG-20260928-handoff-stream-error-says-retry (new, fixed); BUG-20260928-derive-stopped-source-turn-in-progress (from part C, fixed here at the controller's request).
- **Scenarios settled:** RT-session-derive-retry → pass (fixed); RT-provider-error-handoff → pass (fixed); RT-conversation-rewind provenance leg → pass (note only).
- **Paper cuts:** none beyond the fixed ones.
- **Surprises:** an idle `session stop` records a `prompt_interrupted` marker with `stop_reason user_canceled` even when no prompt was running.
- **Suggested next charter:** daemon crash during a derive commit (between receipt and first bind) via a fault-injection build.

### CH-session-fallback-seat — Rafa

- **Ran:** 01:38Z → 01:41Z (box respected: yes)
- **Findings:** the documented account route form (`NAME=value {cmd}`) never launched: the launcher treated the assignment as the executable and the chain exhausted (Blocks-Completion for every account route) → fixed. After the fix: bind on seat two, one `session.fallback.used` with the fingerprint and no command text, one marker, no stop/error, `runtime.effective` = seat two, post-acceptance rate limit writes no fallback event (user session → `handoff`), stop/prompt reloads seat two, route removal restarts on the primary with `fallback_reason: accepted_route_missing`.
- **Bugs filed/updated:** BUG-20260928-route-command-env-prefix-not-launched (new, fixed).
- **Scenarios settled:** RT-session-fallback-chain → pass (fixed).
- **Paper cuts:** `session resume {stopped}` answers `session not attachable` (resume = attach; a prompt is what resumes) — pre-existing rule, also noted by part C.
- **Surprises:** the refused-route marker says `next_action install_cli` for a missing CLI (correct: `use_fallback` is only for rate-limited/unauthenticated refusals).

### CH-role-fallback-boundary — Ada

- **Ran:** 01:41Z → 01:42Z (box respected: yes)
- **Findings:** `roles show auto_title` prints the Command column; `-o json` carries `command` + `command_fingerprint`; the env-prefixed route titled the session after the launcher fix with one `role.fallback.used` (CLI and HTTP agree, no command text anywhere in the daemon log); an accepted-then-failed start (`reject_set` model) stops the chain after attempt 1 with the title unchanged. Memory-controller branch not walked (out of this leg's scope).
- **Bugs filed/updated:** BUG-20260928-route-command-env-prefix-not-launched.
- **Scenarios settled:** MS-background-role-fallback → pass (fixed).

## What Was Fixed

All fixes are uncommitted in the worktree (caller instruction: do not commit).

### BUG-20260928-native-fork-clone-load-refused: Codex native fork child stuck pending
- **Symptom:** every prompt to a Codex fork child fails with `thread … already has an active writer` while the source lives.
- **Root cause:** codex-acp holds the forked thread in the forking process (proved by a stdio probe; `session/close` does not release it); the child bind only fell back to replay for `resource not found`/unsupported.
- **Fix:** `internal/session/derive_native_bind.go` — `nativeCloneLoadFailed`: any `session/load` failure of the clone settles `native_state: failed` and replays the carried context (ADR-003 Risks).
- **Regression test:** `TestForkNativeSeed/…when_the_clone_cannot_load` (`internal/session/manager_derive_test.go`, new "clone held by the forking process" case).
- **Retested:** Codex fork child answers from the carried context, status shows `failed … (replayed the carried context)`; OpenCode/Claude native loads unchanged.

### BUG-20260928-route-command-env-prefix-not-launched: account route commands never launch
- **Symptom:** `fallback chain exhausted … executable file not found in $PATH: QA_ACCOUNT=seat-two`.
- **Root cause:** `localLauncher.PrepareLaunch` split commands naively instead of using `compozyconfig.ParseLaunchCommand`.
- **Fix:** `internal/acp/launch_identity.go` — leading assignments become child environment.
- **Regression test:** `TestLocalLauncherPrepareLaunchForwardsLeadingEnvironment` (`internal/acp/launcher_tool_host_test.go`).
- **Retested:** RT-session-fallback-chain and MS-background-role-fallback walks.

### BUG-20260928-handoff-stream-error-says-retry
- **Symptom:** CLI prints `next_action=retry` for a rate-limited user session that the ledger marks `handoff`.
- **Root cause:** `decorateHandoffAction` skipped `event.Error`.
- **Fix:** `internal/session/derive_handoff.go` rewrites the metadata in `Error` and `Failure.Summary`.
- **Regression test:** `TestDecorateHandoffAction` (`internal/session/derive_test.go`).
- **Retested:** rate limit + auth lapse on a user session; spawned child unchanged.

### BUG-20260928-derive-replay-deleted-child-origin-lost
- **Symptom:** `Continued {src} (--) into {child} (deleted)`.
- **Root cause:** the receipt outcome did not record the origin agent.
- **Fix:** `origin_agent_name` on `store.SessionDerivationOutcome` (`internal/store/types_session_derive.go`), written in `deriveOutcome` (`internal/session/derive_prepare.go`), read in `deriveResultFromReceipt` (`internal/session/derive_commit.go`).
- **Regression test:** `TestContinueSession` retry case (`internal/session/manager_derive_test.go`).
- **Retested:** fresh key, delete, retry → `(retry-source)`.

### BUG-20260928-derive-stopped-source-turn-in-progress (filed by part C)
- **Symptom:** preview/derive of any stopped (or never-prompted) session reports `source_turn_in_progress: true` and appends the aborted-turn note.
- **Root cause:** `lastSettledTurn` counted lifecycle-only turn ids (hook dispatch, stop escalation) as open turns.
- **Fix:** `internal/session/derive_cut.go` — only prompt turns (carrying a prompt-stream event) can be "in progress".
- **Regression test:** `TestLastSettledTurn/Should_not_flag_lifecycle-only_turns_…` (`internal/session/derive_test.go`).
- **Retested:** stopped source → `false` (988 replay bytes instead of 1351), unprompted → `false`, running turn → `true`.

### BUG-20260928-claude-inspection-close-timeout (pre-existing, surfaced by the walk)
- **Symptom:** intermittent `Provider configuration is unavailable: acp: model "claude-haiku-4-5-20251001" is unavailable in config option "model"` on a Claude child's first prompt.
- **Root cause:** claude-agent-acp answers the inspection `session/close` after the 1 s budget; the expired budget failed the whole model-catalog inspection.
- **Fix:** `internal/acp/client_control.go` — an expired close budget is not an inspection failure.
- **Regression test:** `TestInspectSessionConfigOptionsDoesNotMutateTheACPNewSession/…outlasts_its_budget` (`internal/acp/client_start_contract_test.go`, helper scenario `config_options_slow_close` in `client_test_support_test.go`).
- **Retested:** no further `refresh_failed … close inspected session` after the rebuild; later Claude binds resolved the model first time.

Verification run after the last change: `go test ./internal/session/ ./internal/acp/ ./internal/store/ -count=1` (2074 passed); `go test -tags integration ./internal/daemon/ -run 'TestDaemonE2ESessionContinueCLI|TestDaemonE2ESessionForkCLI|TestDaemonE2EAgentFallbackChain|TestAutoTitleRoleIntegration' -count=1` (13 passed); `GOOS=windows go build ./internal/acp/ ./internal/session/`; golangci-lint on the three packages reports no issue on the changed lines (the packages carry pre-existing findings). `make gate` not run (caller instruction; owed by the delivery step).

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Rafa | J-15 continue, unknown agent | "It says no agent named nope but not the code the docs mention" | dull | watching (JSON has `agent_not_found`) |
| Rafa | J-15 logs by session | "The session id already says which workspace; why do I need to cd?" | dull | deferred |
| Théo | J-15 retry after delete | "Continued … (--) — who was it from?" | sharp | fixed (BUG-20260928-derive-replay-deleted-child-origin-lost) |
| Théo | J-15 rate limit | "The CLI says retry but the session says hand off" | sharp | fixed (BUG-20260928-handoff-stream-error-says-retry) |

## Runtime Errors Observed

- `daemon.model_catalog.refresh_failed … close inspected session … context deadline exceeded` (claude) — filed and fixed as BUG-20260928-claude-inspection-close-timeout.
- `daemon.model_catalog.refresh_failed` for `pi` (no model discovery configured) — unrelated lab noise, not filed.
- `skills: verification warning` for operator skills under `~/.agents/skills` — lab noise from the operator home.

## Human Verifications Needed

None.

## Decisions for a Human

- **Scenario text vs. adapter reality:** `RT-session-derive-native-fork` step 5 and the charter still say Claude/Codex fork by replay; today's adapters all fork natively. Recommend rewording step 5 to "an adapter without `session/fork` (or a stopped/busy source) returns `seed: replay`". Not changed here (scenario body is owned by qa-report).
- **Codex native fork is always a failed native load while the source lives** (codex-acp limitation): the child now works via replay, but the fork dialog/preview still promises the native line for Codex. Options: (a) keep (truthful after the fact, status says `failed … replayed`); (b) gate native fork per provider when the source process stays alive; (c) upstream codex-acp change. Recommendation: (a) now, report upstream.

## Final Status

**Verdict: PASS** (all walked rows Pass or Fixed-and-retested).

Part B1 scoped round complete: 7 matrix rows terminal — 2 Pass, 5 Fixed (plus the controller-requested stopped-source fix). Findings by impact tier: Blocks-Completion 2 (native fork clone load, account route launch) + 1 intermittent (Claude inspection close), Trust-Damage 3 (handoff stream text, deleted-child origin, stopped-source in-progress), all fixed with regression tests and re-walked on the rebuilt daemon; no open finding from this part. Limitations: the undispatched-first-message crash window is not reachable from the CLI (automated coverage only); the pre-feature root of Lab B was not re-walked after the stopped-source fix; strict auditor C14 (local gate evidence) stays open because `make gate` was out of this part's scope. Lab torn down (see teardown in `memory/qa-execution.md` Part B1).
