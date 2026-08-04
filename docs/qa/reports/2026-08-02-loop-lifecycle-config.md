# QA Run Report — 2026-08-02 — loop-lifecycle-config

- **Scope:** Task 01 adds structured configuration paths and validation for Loop lifecycle defaults.
- **Cadence tier:** targeted
- **Build:** working-tree fingerprint `e2cacea641b8f8d580c4b16147d21faf86301172` · **Environment:** isolated local daemon on port `60606`, workspace `ws_e71fe16302b0917b`, global schema version `39`
- **Started:** 2026-08-02T07:08:11Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-loop-lifecycle-config-cli |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-config-file-overrides |

## Flows in Scope

- `J-tune-loop-lifecycle-defaults` — tune global Loop lifecycle policy safely (`../journeys/J-tune-loop-lifecycle-defaults.md`)
- `J-configure-and-run-loop` — adjacent reusable Loop-config file canary (`../journeys/J-configure-and-run-loop.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-lifecycle-config-cli | J-tune-loop-lifecycle-defaults / LP-loop-lifecycle-config-cli | Ada | Feature Tour | Fixed | BUG-20260802-loop-lifecycle-config-unsupported | Task 01 checkpoint |
| 2 | CH-loop-config-file-overrides | J-configure-and-run-loop / LP-loop-config-file-snake-case | Bruno | Feature Tour | Pass | | 38b2d40 |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-loop-lifecycle-config-cli — Ada — initial walk

- **Ran:** 2026-08-02T07:13:43Z → 2026-08-02T07:14:00Z (box respected: yes)
- **Findings:** Blocks-Completion — the first documented lifecycle path is rejected as unsupported.
- **Bugs filed/updated:** BUG-20260802-loop-lifecycle-config-unsupported
- **Scenarios settled:** LP-loop-lifecycle-config-cli → fail pending fix
- **Paper cuts:** none; the CLI error itself was direct and actionable, but the advertised path was unreachable.
- **Surprises:** the unit-level path inventory did not match the CLI's live policy registry.
- **Suggested next charter:** re-run this charter from a fresh config state after the registry fix.

### CH-loop-lifecycle-config-cli — Ada — fix replay

- **Ran:** 2026-08-02T07:20:00Z → 2026-08-02T07:23:00Z (box respected: yes)
- **Findings:** none after the fix; every documented mutable path persisted and read back.
- **Bugs filed/updated:** BUG-20260802-loop-lifecycle-config-unsupported → fixed
- **Scenarios settled:** LP-loop-lifecycle-config-cli → pass
- **Paper cuts:** none.
- **Surprises:** lifecycle config correctly reports daemon restart-required instead of claiming a
  live apply.
- **Suggested next charter:** exercise these seeded defaults when retry and wait execution lands.

### CH-loop-config-file-overrides — Bruno — adjacent canary

- **Ran:** 2026-08-02T07:23:00Z → 2026-08-02T07:24:30Z (box respected: yes)
- **Findings:** none; JSON preview, YAML persistence, nested checks, and non-mutating strict
  rejection all behaved as documented.
- **Bugs filed/updated:** none.
- **Scenarios settled:** LP-loop-config-file-snake-case remains pass.
- **Paper cuts:** none.
- **Surprises:** none.
- **Suggested next charter:** retain the existing reusable-file regression in future Loop CLI
  changes.

## What Was Fixed

- Shared the native-tool path policy with the CLI classifier so one canonical registry owns
  agent-mutable path availability and value kinds.
- Added a public CLI regression covering every lifecycle path and its persisted readback.

## Paper Cuts

None.

## Runtime Errors Observed

None.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

The public write boundary caught a registry split that package-level config-policy tests could not
expose. Agent-manageable config needs one policy source across native tools and CLI.

## Final Status

- **Exit gate:** `make gate` escalated to the full `make verify` suite and passed at
  2026-08-02T08:19:19Z for fingerprint `e2cacea641b8f8d580c4b16147d21faf86301172`.
- **Evidence audit:** strict audit passed with zero blockers and zero warnings.
- **Teardown:** `teardown.json` records `"clean": true`, zero survivors, and daemon PID `52973`
  stopped at 2026-08-02T08:20:36Z.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 ·
  Cosmetic 0.
- **Coverage:** 2 journeys walked, 2 passed after the lifecycle-path fix, 0 skipped, 0 blocked.
- **Verdict:** ready — the exercised lifecycle configuration surface and its adjacent config-file
  path pass behavioral, automated, evidence-integrity, and teardown checks.
