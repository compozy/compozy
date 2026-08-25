# QA Run Report — 2026-08-24 — ENG-147 TTL cleanup

- **Scope:** State-aware TTL cleanup for governed spawned sessions, including clean terminal projections, parent wake classification, timeout markers, lifecycle hooks, and lease/process teardown.
- **Cadence tier:** targeted
- **Build:** working tree after atomic-stop review · **Environment:** isolated Go lifecycle harness; no full-monorepo gate by request
- **Started:** 2026-08-24T22:08:00-03:00 · **Status:** ready with blocked public-surface verification

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-018 |

## Flows in Scope

- `J-15` — An agent drives and reads sessions deterministically over CLI, HTTP, or UDS (`../journeys/J-15-operate-session-via-cli-api.md`).
- `RT-spawn-ttl-cleanup` — Reap settled and in-flight governed children with truthful terminal projections (`../scenarios/RT-spawn-ttl-cleanup.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-018 | J-15 / RT-spawn-ttl-cleanup | Ada | Feature Tour | Blocked (needs human verify) | Isolated ACP provider required for a public CLI/HTTP/UDS walk | |
| 2 | CH-018 | J-respond-to-agent-attention / RT-session-spawn-wake | Ada | Feature Tour | Blocked (needs human verify) | Same provider-backed stopped-child wake walk required after TTL classification change | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-018 — Ada

- **Ran:** focused automated lifecycle walk · **box respected:** yes
- **Findings:** settled TTL cleanup classified as completed; active prompt classified as timeout; the
  lifecycle contract recorded one stopped notifier and one clean parent wake with no timeout marker.
- **Bugs filed/updated:** none
- **Scenarios settled:** blocked-verify for both public-surface legs; automated evidence is recorded below
- **Paper cuts:** none observed yet
- **Surprises:** the runtime session manager can classify the TTL cause atomically under its lifecycle lock;
  transport-only adapters conservatively retain timeout classification when prompt state is unknown.
- **Suggested next charter:** supply an isolated ACP provider and re-walk `RT-spawn-ttl-cleanup` plus
  `RT-session-spawn-wake` through CLI, HTTP, and UDS.

## What Was Fixed

### ENG-147: Settled child TTL cleanup was reported as a prompt timeout

- **Symptom:** a child that had already settled was shown as failed and woke its parent with a failed outcome when TTL cleanup ran.
- **Root cause:** the reaper treated every `ttl_expired` reason as `CauseTimeout`, regardless of the live prompt state.
- **Fix:** classify TTL cleanup atomically at the session stop transition: a settled prompt maps to
  completed, an in-flight prompt maps to timeout, and transport-only adapters use a conservative
  timeout fallback; preserve the reaper origin on clean completion.
- **Regression test:** `internal/daemon/spawn_reaper_test.go`; `internal/session/stop_reason_test.go`; `internal/session/manager_lifecycle_contract_test.go`
- **Retested:** review findings addressed; focused daemon/session lifecycle tests pass.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Ada | J-15 / TTL cleanup | Structured lifecycle output must distinguish a clean TTL stop from a timeout without requiring transcript inspection. | dull | watching |

## Runtime Errors Observed

- None in focused lifecycle tests; public-surface walk pending.

## Human Verifications Needed

- [ ] Run a live provider-backed CLI/HTTP/UDS walk for both a settled `done`/`end_turn` child and a
  genuinely in-flight prompt at TTL. Confirm list/status/events, parent wake badge, timeout marker,
  lease release, process teardown, and exactly-once lifecycle hooks. This requires an isolated ACP
  provider and a human operator; no provider-backed public surface was available in this review run.

## Decisions for a Human

None.

## Learnings

- TTL expiry is a cleanup reason, not a failure classification by itself; prompt activity is the deciding runtime fact.

## Final Status

- **Exit gate (focused automated suite):** pass — `go test -race ./internal/daemon -run 'TestSpawnReaper' -count=1` (9 passed); `go test -race ./internal/session -run 'Test(ClassifyStopReason|SpawnTTLStopCauseUsesPromptStateAtLifecycleTransition|CompletedSpawnReapKeepsCleanTerminalProjection|PromptActivitySupervisorTimeoutCancelsThenStopsSession)$' -count=1` (22 passed); new-code golangci-lint (`--new-from-rev origin/main`) reported 0 issues. Full-monorepo gates were not run per the ENG-147 request.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 0/2 public journeys walked; both are `blocked-verify` with exact provider-backed instructions. Automated lifecycle evidence covers the runtime invariant.
- **Verdict:** ready with blocked public-surface verification — do not treat the blocked rows as a provider-backed pass.
