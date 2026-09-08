# QA Run Report — 2026-09-05 — Sessions stability: live steer and truthful stop (task_01 + task_02)

- **Scope:** `.compozy/tasks/sessions-stability` task_01 (live steer end-to-end) and task_02 (truthful stop and boot truth) on branch `session-improvs`.
- **Cadence tier:** targeted (cli, api, web, runtime, provider)
- **Build:** working tree based on `13f4f3dbdfe86b1226ee621f63f98066199aae5e` · **Environment:** isolated lab `compozy-sessions-stability-task01-02-20260905-154017-502928` (fresh `COMPOZY_HOME`, HTTP 64281, Vite web dev on 5177 proxied to the lab daemon, operator Claude Code login preserved)
- **Started:** 2026-09-05T15:40:17Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Returning user | desktop / wifi-fast / en-US | CH-sessions-stability-steer, CH-sessions-stability-stop |

## Flows in Scope

- `J-13` — Send a prompt and redirect a running agent without losing its work (`RT-018`, `RT-019`, `RT-session-live-steer`).
- `J-15` — Stop a session and trust the final state (`RT-session-native-stop`, `RT-session-prompt-cancel`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-sessions-stability-steer | J-13 / RT-session-live-steer | Théo | Feature Tour | Pass | | |
| 2 | CH-sessions-stability-steer | J-13 / RT-018 | Théo | Feature Tour | Pass (stream, busy default, queue) · provider_error journey not walked | | |
| 3 | CH-sessions-stability-stop | J-15 / RT-session-native-stop | Théo | Feature Tour | Pass (CLI stop --wait, prompt-cancel, crash restart, Web Stopping) · native-tool/UDS/unverifiable-kill/Daytona not walked | | |
| 4 | CH-sessions-stability-stop | J-15 / RT-session-prompt-cancel | Théo | Feature Tour | Pass | | |

## Session Debriefs

### CH-sessions-stability-steer — Théo

- **Ran:** 2026-09-05T15:46:00Z → 2026-09-05T16:20:00Z (box respected: yes)
- **Findings:**
  - Real provider steer (Claude Code 2.1.261, `claude-sonnet-5`): a second `compozy session prompt` during a live 40-item turn returned `disposition: steering`, `steer_delivery: injected`, `turn_id` = the live turn; the transcript carries `transcript_marker.prompt_steered` ("Steering delivered into the live turn.") inside the same turn, and the agent answered `STEER_ACK_7731` with `done end_turn` on that same turn id (lab events seq 79–88). No new turn boundary, no cancel.
  - Web Enter on a live Claude turn (`claude-web`): the composer answered inline with "Steering — delivered into the live turn `injected`", the field emptied, Stop stayed primary, and the transcript later carried the `prompt_steered` marker (seq 379, same turn).
  - No-capability agent (acpmock `stubborn`, `busy_input.steer_capability: none`): an unmarked busy send resolved the daemon default (`steer`) and reported `steer_delivery: interrupt_fallback`; the old turn ended `cancelled` with markers `prompt_steered` ("Steering interrupted and replaced the active turn.") + `prompt_cancel`, the replacement ran on a fresh turn and answered `STUBBORN_ACK`. The Web shows "Interrupted and replaced — this agent can't take guidance mid-turn `interrupt_fallback`".
  - Modifier (Cmd+Enter) during a held turn queued the follow-up: "Queued #1 — runs after the current turn inq-…" and `compozy session input list` shows the entry with `mode: queue`, `delivery: after_turn`.
  - `session status` answers `busy_input.default_mode` / `steer_capability` before any send (`steer_ext` for Claude, `none` for acpmock).
- **Bugs filed/updated:** None.
- **Scenarios settled:** RT-session-live-steer → pass; RT-018 → blocked-verify (stream + busy default + queue passed; the provider auth-lapse/rate-limit `provider_error` journey needs a forced live auth lapse and was not walked).
- **Paper cuts:** The legacy `delivery` field still reads `interrupt_then_prompt` on an injected steer; the new `steer_delivery` field carries the truth (retained for the v0.4 compatibility window per SD-013).
- **Surprises:** Claude Code announces `_session/steering`, so the shipped `none` table entry is overridden by the handshake exactly as ADR-010 orders precedence.
- **Suggested next charter:** Repeat the steer walk on Codex and Cursor binaries once their ACP builds announce steering.

### CH-sessions-stability-stop — Théo

- **Ran:** 2026-09-05T15:48:00Z → 2026-09-05T16:16:00Z (box respected: yes)
- **Findings:**
  - `compozy session stop <id> --wait -o json` on the acpmock agent returned `{"state":"stopped","verified":true,"escalated":true,"stop_cause":"user_requested","phase":"forced","stopped_after":"87.739ms"}`; the driver process was gone afterwards and `session.stop_escalated` (scope session, phase forced) plus `session_stopped` were persisted.
  - `compozy session prompt-cancel` mid-turn returned `{"outcome":"canceled","turn_id":…}`; the session stayed `active`/`idle`/`attachable`, a follow-up prompt ran (`STUBBORN_ACK`), and an idle cancel returned `nothing-in-flight` with exit 66 in human output.
  - Cancel-ignoring turn (`hold_ignoring_cancel` acpmock control, new in this delivery): Web Stop flipped the primary control to "Stopping…" (guarded pill, Steer/Interrupt hidden, Queue kept) and held it through the 10 s cooperative grace; the daemon recorded `session.stop_escalated` scope turn, phase forced, `elapsed_ms: 10000`, then the turn settled and the session returned to idle/promptable.
  - Crash simulation: `kill -9` on the daemon during a held turn, restart → the session reads `stopped`, badge `failed`, health `dead`, `verified: true`, with `session_stopped stop_reason: agent_crashed` and an `error` event; no phantom `active` row and no orphaned driver process.
- **Bugs filed/updated:** None.
- **Scenarios settled:** RT-session-prompt-cancel → pass; RT-session-native-stop → blocked-verify (CLI/HTTP/Web stop truth, prompt-cancel and crash-restart passed; `compozy__session_stop` from an agent session, UDS parity, the unverifiable-kill branch and Daytona remote proof were not walked in this bounded lab).
- **Paper cuts:** None.
- **Surprises:** A session-scope stop on an ACP agent always escalates to the forced phase once the turn quiesces, because ACP agents do not exit on cancel; `escalated: true` is therefore the honest reading for acpmock even when the ladder settles in under 100 ms.
- **Suggested next charter:** Native `compozy__session_stop` + UDS parity walk inside a governed agent session, plus the Daytona remote-exit proof once a sandbox lab is available.

## What Was Fixed

### Session metadata size regression (lint)

- **Symptom:** the two new stop-truth booleans pushed `store.SessionMeta` to 512 bytes, tripping 54 `gocritic hugeParam` findings across `internal/session`.
- **Fix:** grouped `RuntimeFailure` and `RuntimeSelection` into the optional `*SessionRuntimeBindingState` embed with value-semantics accessors (the same pattern the struct already uses for execution location and provider execution state); the flat JSON contract is unchanged.
- **Regression test:** existing `internal/store`, `internal/session` and `internal/observe` suites under `go test -race`.

### Cancel-ignoring test agent

- **Symptom:** the acpmock driver honored `session/cancel` in every step, so no lab could exercise the escalation ladder end-to-end.
- **Fix:** new `hold_ignoring_cancel` driver-control action (validated like `delay`) that keeps the turn open for `delay_ms` while ignoring prompt cancellation.
- **Regression test:** `internal/testutil/acpmock/driver_cancel_test.go` — `TestDriverHoldIgnoringCancelKeepsTurnOpen`.

## Visual Contract Evidence

Bundles (reference artboard state ↔ implementation capture, diagnostic diff, review) validated with `validate-visual-contract.mjs`:

- task_01 VC-01 injected (live Claude, web), VC-02 pending injection (canonical story), VC-03 interrupt fallback (live acpmock, web), VC-04 queued with position (live, web), VC-05 gate refusal (canonical story), VC-06 Follow-up behavior control (Settings story) → PASS, 0 blocking divergences.
- task_02 VC-01 Stopping… (live, cancel-ignoring turn), VC-02 stopped confirmed (live) → PASS, 0 blocking divergences.
- Location: `.compozy/tasks/sessions-stability/evidence/visual/task_0{1,2}/VC-0N/` (local task package).

## Paper Cuts

- Legacy `delivery: interrupt_then_prompt` alongside `steer_delivery: injected` (compatibility window; documented).

## Runtime Errors Observed

An early fixture used an invalid step kind (`text`), which the daemon surfaced as a truthful bind failure (`session.start.driver_start_failed`) rather than a phantom active session; corrected to `assistant`.

## Human Verifications Needed

- Steer on Codex / Cursor binaries (no steering announcement observed; falls back truthfully).
- `compozy__session_stop` from a governed agent session and UDS parity.

## Decisions for a Human

None.

## Evidence

- Lab: `/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/` (`bootstrap-manifest.json`, `evidence/walk*.json`, `evidence/web-*.jsonl`, `evidence/screenshots/*.png`, `evidence/storybook/*.png`, `evidence/reference/*.png`).
