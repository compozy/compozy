# QA Run Report — 2026-08-24 — ENG-148 agent-session Goal control

- **Scope:** ENG-148 — authenticated typed Goal control for agent sessions across native, HTTP, UDS, CLI, and Web-backed session surfaces
- **Cadence tier:** targeted
- **Build:** final focused ENG-148 candidate (`/tmp/compozy-eng148-final`, `/tmp/acpmock-eng148-final`) · **Environment:** fresh isolated QA lab (manifest recorded below)
- **Started:** 2026-08-25T01:40:09-03:00 · **Completed:** 2026-08-25T02:34:24-03:00 · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-agent-session-control |

## Flows in Scope

- `J-29` — operate and recover a Goal without UI-only shortcuts (`../journeys/J-29-operate-and-recover-goal.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-agent-session-control | J-29 / GL-agent-session-control | Ada | Error Guessing Tour | Pass | | |
| 2 | CH-agent-session-control | J-29 / ET-web-session-goal-strip | Ada | Error Guessing Tour | Pass | | |
| 3 | CH-agent-session-control | J-29 / GL-025, GL-026, GL-034 | Ada | Error Guessing Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **CLI:** Final binary set a Goal with typed `acpmock/goal-e2e-judge` runtime, then paused and resumed a second Goal. JSON returned run IDs and stable lifecycle outcomes.
- **HTTP:** Final binary returned `202` for typed Goal set and `200` for the matching Goal read. Parent session headers controlled a child; a child attempting to control its parent returned `403 goal_caller_unauthorized`.
- **UDS:** Unix-domain socket read returned the same Goal snapshot and content type as HTTP.
- **Runtime/isolation:** Focused race tests covered origin binding, workspace/lineage rejection, cyclic lineage rejection, provider-only runtime selection, and eight concurrent child Goals with unique run/network/session bindings.
- **Web:** `agent-browser` selected the isolated workspace, opened `/agents/qa_general/sessions/sess-8539c81f6527e989`, and rendered the `Goal status` region. The expanded strip showed `Contract`, `Run`, `Context`, `Last verdict`, and `Node`; the head action was `Clear goal`. Evidence: `qa/screenshots/goal-status-strip.png` and `qa/screenshots/goal-status-expanded.png` under the lab output.
- **Evidence index:** `/Users/pedronauck/dev/qa-labs/compozy-eng-148-agent-session-control-20260825-014009-304323-lab/qa-artifacts/qa/journey-log.jsonl`.
- **Teardown evidence:** the bootstrap manifest's `TEARDOWN_COMMAND` completed; `/Users/pedronauck/dev/qa-labs/compozy-eng-148-agent-session-control-20260825-014009-304323-lab/qa-artifacts/qa/teardown.json` records `"clean": true`, with no lab processes left running.

## Human Verifications Needed

- [x] Browser walk completed on the final candidate; Goal strip and expanded details passed. Screenshot evidence is recorded in the lab output.

## Final Status

<!-- Written LAST, after the focused exit gate and strict QA auditor. -->

- **Exit gate:** focused ENG-148 checks passed; `make gate-full`/`make verify` intentionally not run per the task instruction that CI owns the full-monorepo gate.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** GL-agent-session-control, ET-web-session-goal-strip, GL-025, GL-026, GL-034 — all walked and recorded as `pass`.
- **Strict auditor:** `/Users/pedronauck/dev/qa-labs/compozy-eng-148-agent-session-control-20260825-014009-304323-lab/qa-artifacts/qa/qa-audit-report.json` reports only C14 (missing full `make verify` evidence), which is the explicitly deferred CI-owned gate.
- **Verdict:** pass-with-blocked-decision — targeted QA is green; full-monorepo verification is explicitly deferred to CI.
