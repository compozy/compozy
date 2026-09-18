# QA Run Report — 2026-09-17 — clarify-keepalive

- **Scope:** clarify-keepalive tasks 01–03 slice (unbounded-by-default clarification waits + `_compozy/clarify_ping` keepalive) plus the task_05 fix-loop repair of the `config set` operating surface.
- **Cadence tier:** targeted
- **Build:** `7b64ab14c` + uncommitted worktree (tasks 01–04 slice + task_05 one-entry allowlist fix in `internal/config/tool_surface.go`) · **Environment:** isolated QA lab, `COMPOZY_HOME=/tmp/compozyqa-88565633a518/runtime`, daemon `127.0.0.1:39383`, lab binary built from the same worktree (`/tmp/compozyqa-88565633a518/compozy`)
- **Started:** 2026-09-17T23:32:26Z · **Status:** closed · **Teardown:** `teardown.json` `"clean": true` (2026-09-17T23:47:32Z, no survivors)

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | admin settings | desktop / wifi-fast / en-US | CH-clarify-policy-matrix |
| Théo | live clarification | desktop / wifi-fast / en-US | CH-clarify-long-wait-keepalive |
| Ada | races | desktop / wifi-fast / en-US | CH-clarify-race-termination-sweep |
| Bruno | extension question | desktop / wifi-fast / en-US | CH-clarify-extension-inheritance |

## Flows in Scope

- `J-administer-runtime-settings` — policy lifecycle (omitted/`0s`/finite/invalid + restart-required + last-valid-kept)
- `J-answer-agent-requests` — unbounded wait past 60s with pings, late answer, finite fallback, races, extension inheritance, CLI parity
- Canary `J-15-operate-session-via-cli-api` — shared session/CLI surfaces (full `internal/cli` + `internal/config` suites green)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-clarify-long-wait-keepalive | J-answer-agent-requests / RT-session-clarification-roundtrip | Théo | Interrupt | Pass | | |
| 2 | CH-clarify-policy-matrix | J-administer-runtime-settings / MS-clarify-timeout-policy | Dora | Garbage | Fixed | BUG-20260917-clarify-timeout-config-set | worktree (uncommitted, this task) |
| 3 | CH-clarify-race-termination-sweep | J-answer-agent-requests / RT-session-clarification-roundtrip | Ada | Multi-Tab | Pass | | |
| 4 | CH-clarify-extension-inheritance | J-answer-agent-requests / RT-session-clarification-roundtrip | Bruno | Feature | Pass | | |
| 5 | canary J-15 | CLI/API session surfaces | — | — | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-clarify-long-wait-keepalive — Théo

- **Ran:** 2026-09-17T23:34:01Z → 23:35:16Z live (E2E harness, own isolated daemons; box respected: yes)
- **Findings:**
  - Unbounded pending shows `"deadline": null` via `session clarify pending -o json`; the blocked `compozy__clarify` call stays open across a 65s operator wait; daemon terminal debug line correlates `last_ping_seq >= 2` (immediate + 30s + 60s ticks); late `--choice 1` returns the exact answer `{"choice": 0, "text": "", "fallback": false}` — never the sentinel. (E2E-001, 71.29s)
  - Finite control (2s live substitute for 5m, same code path; 5m pinned at broker/boot level): no answer → fallback sentinel `{choice: null, text: "", fallback: true}` + open-list absence (`timed_out`); timely `--choice 2` resolves `{choice: 1, no fallback}`. (E2E-002, 3.03s)
  - Agent used: fixture-backed mock (`attention-agent`, acpmock — no extension handler, correct per spec: unknown extension notifications are fire-and-forget and succeed client-side; Compozy-side deadline stays authoritative). The 30s margin vs real-provider idle limits could not be observed with a mock — recorded as deferred follow-up, not a defect.
- **Bugs filed/updated:** none in this charter
- **Scenarios settled:** RT-session-clarification-roundtrip → pass (long-wait legs)
- **Paper cuts:** none
- **Surprises:** none — first pass held
- **Suggested next charter:** real-provider idle-limit read (see Decisions for a Human)

### CH-clarify-policy-matrix — Dora

- **Ran:** 2026-09-17T23:33:30Z → 23:38:55Z across two lab-daemon generations (box respected: yes)
- **Findings:**
  - `config set tools.clarify.timeout` denied pre-fix (`cli: config path ... is not supported by config set`) though `_dx.md`/`_spec.md` name it as the operating surface — filed BUG-20260917-clarify-timeout-config-set (Friction), fixed in-loop with a one-entry allowlist addition + contract test, re-walked green.
  - Post-fix matrix (exact commands/outputs in lab logs `policy-P1..P6`, `policy-Q1..Q4`): omitted → `0s`; explicit `0s` → `0s` (indistinguishable after restart); `5m` → `5m0s`; bounds `1s` → `1s`, `24h` → `24h0m0s`; `soon` → decode-layer error naming key+value; `25h`/`500ms`/`-5s`/`99h` → exact `tools.clarify.timeout must be between 1s and 24h, or 0s for no expiration: <value>`.
  - Restart-required: mid-flight file flip surfaces `config reload -o json` → `"applied": false, "restart_required": true, "next_action": "restart-daemon"` + apply-history `blocked` warn entry; after restart, reload → `"applied": true, "lifecycle": "live", "restart_required": false`.
  - Last-valid-kept: invalid file makes even `daemon stop` refuse with the exact policy error while the daemon keeps serving the prior generation; `config set 99h` fails and `config get` still returns the prior value; `config unset` deletes the key (`deleted: true`, live) and restart resolves omitted → `0s`.
  - Creation-time pinning (mid-wait flip) verified at the broker seam by UT-006 (green in the `-race` daemon run); no live mid-wait flip was attempted — same-code-path evidence cited, not re-walked live.
- **Bugs filed/updated:** BUG-20260917-clarify-timeout-config-set (fixed, verified same session)
- **Scenarios settled:** MS-clarify-timeout-policy → pass
- **Paper cuts:** `config unset` leaves an empty `[tools.clarify]` table header in config.toml (dull; parses as omitted, no behavior impact) — watching, not filed.
- **Surprises:** `config get` reads desired file state, not the active generation — the charter's "prove across CLI output" step needs `config reload`/apply-history for the active-generation half. Noted in Learnings.
- **Suggested next charter:** none — matrix closed.

### CH-clarify-race-termination-sweep — Ada

- **Ran:** 2026-09-17 (suite evidence, `-race` daemon run 3.964s + E2E CLI walks; box respected: yes)
- **Findings:**
  - Answer/expiry race settles exactly once under `-race` (`TestClarifyBridgeAnswerExpiryRace`); post-terminal pings silenced (UT-012), failing pings never disturb the wait (UT-011), second same-session ask conflicts naming the live request (UT-007), unknown/finished answers denied deterministically (UT-013 legs) — all green.
  - CLI parity: `pending -o json` carries `deadline: null` with per-item `session_id` (E2E-001 assertion), one-based `--choice` translates at the CLI boundary (CLI suite + E2E answers); second-ask conflict, cross-workspace denial, and >4-choices validation ride the unchanged deny paths (full CLI suite green, no spec code touches them).
  - No live multi-surface race (CLI vs HTTP/UDS concurrently) was staged — race-once is proven at the owning seam under `-race`; concurrent-surface staging is a named follow-up, not a blocker.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-session-clarification-roundtrip → pass (race/termination legs)
- **Paper cuts:** none

### CH-clarify-extension-inheritance — Bruno

- **Ran:** 2026-09-17 (seam evidence + extension suite 4.727s; box respected: yes)
- **Findings:**
  - `handleClarifyAsk` (`internal/extension/host_api_clarify.go:152`) calls the same `broker.Ask` as the native tool — identical wait, pinning, ping, and answer contract; no manifest/permission/SDK change (no `internal/extension` prod file touched by the spec).
  - Extension delegation suite green (scope derivation, foreign-invocation rejection, origination-cancel, failure classification); unbounded/finite wait semantics inherit from the bridge suites.
  - No live extension-host ask was staged (no extension host in the lab); inheritance is by shared-seam construction, stated as the boundary.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-session-clarification-roundtrip → pass (inheritance legs)
- **Paper cuts:** none

## What Was Fixed

### BUG-20260917-clarify-timeout-config-set: config-set denial
- **Symptom:** `compozy config set tools.clarify.timeout 5m --scope user -o json` → `cli: config path "tools.clarify.timeout" is not supported by config set`.
- **Root cause:** policy admission/validation (tasks 01–03) never registered the path in `agentMutableConfigKinds`; the mutation classifier fell through to denial.
- **Fix:** one constant + one `ConfigValueDuration` map entry in `internal/config/tool_surface.go` (uncommitted worktree, per delivery policy commits belong to another surface).
- **Regression test:** new `Should allow clarify timeout mutation` classification case (denied before, allowed after); full `internal/cli` (2.375s) + `internal/config` (1.235s) suites green; live Q1–Q4 re-walk green.
- **Retested:** Dora matrix (all shapes) + canary CLI/config suites; E2E-001/E2E-002 re-confirmed unaffected.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Dora | J-administer-runtime-settings / unset | "unset left an empty `[tools.clarify]` header behind" | dull | watching (parses as omitted; no behavior impact) |

## Runtime Errors Observed

- None during walks. Pre-existing, unrelated: `make gate`'s `codegen-check` lane fails on the stale Daytona sidecar asset (recorded in task_01/task_03 memory, untouched by this spec) — see Final Status.

## Human Verifications Needed

- [ ] Real-provider idle-limit read: ask a clarification under the unbounded policy from a live Claude/Codex (or other provider) agent, wait 90s without answering, confirm the tool call survives on 30s pings and note whether the 30s margin was needed; a provider keeping its own shorter timeout is documented behavior, not a defect. Follow-up owner: next QA cycle touching provider variance (ADR-001 Open Question).
- [ ] Live multi-surface race staging (CLI answer vs HTTP cancel vs session stop concurrently) if a future change touches the terminal path; current race-once evidence is seam-level under `-race`.

## Decisions for a Human

None — the one reproduced defect passed the fix-loop governor (spec-mandated surface, one-allowlist-entry blast radius, contract test + full affected suites + live re-walk) and was repaired in-loop. The two items above are deferred observations with named follow-ups, not escalations.

## Learnings

- `config get` reports desired file state, not the active daemon generation — policy-matrix charters must cite `config reload`/`apply-history` (or a live pending projection) for the active half of a restart-required claim.
- The spec's operating surfaces (`_dx.md` transcripts) are the cheapest QA oracle for classification seams: diffing the transcript's `config set` against the allowlist would have caught this in task_01.
- `daemon start --foreground --exit-when-orphaned` self-terminates when detached; manual lab daemons must use background `daemon start`. E2E harness runs remain the reliable 60s+ wait vehicle (own daemons, proper process groups).

## Final Status

- **Exit gate:** `make gate` → FAIL at `codegen-check` only: Daytona sidecar asset stale (pre-existing on pristine HEAD per task_01/task_03 memory; zero `internal/sandbox` files touched — verified). Spec-owned lanes green: `make go-lint` 0 issues; `make boundaries` respected; `check-cli-docs.sh` up to date; `go test ./internal/cli/ ./internal/config/` ok; `-race` daemon/session/acp/extension clarify suites ok; integration E2E journeys PASS (74.447s).
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 1 (fixed) · Cosmetic 0
- **Coverage:** 1/4 charters live-walked (Dora policy matrix); Théo ran the E2E harness, Ada and Bruno produced seam/suite evidence; MS settled pass; RT pass covers only harness-observed legs with the extension-inheritance and multi-surface race walks outstanding.
- **Taxonomy:** journeys (J-answer-agent-requests, J-administer-runtime-settings, J-15 canary) and functional checks walked; edge/error legs covered at the owning seams with the live multi-surface race and extension-host walks outstanding; experiential skipped (no UI surface changed — zero-deadline rendering is owned by clarify-timeout); cross-cutting consistency swept for stale bounded wording.
- **Verdict:** ready on MS/policy legs; RT re-walk outstanding for the extension-inheritance and multi-surface race legs.
