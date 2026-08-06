# QA Run Report — 2026-08-05 — Review findings managed skills

- **Scope:** Review remediation for managed skill loading: hosted launch cleanup, ACP activation failure handling, managed-session CLI guidance, and operator skill access
- **Cadence tier:** targeted
- **Build:** working tree · **Environment:** fresh CLI-only lab from the QA bootstrap manifest
- **Started:** 2026-08-06T02:08:47Z · **Completed:** 2026-08-06T02:14:55Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-managed-session-skill-loading |

## Flows in Scope

- `J-load-skill-in-managed-session` — load an omitted skill through the native seam and verify the operator CLI independently (`../journeys/J-load-skill-in-managed-session.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-managed-session-skill-loading | J-load-skill-in-managed-session / ET-managed-session-skill-loading | Ada | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-managed-session-skill-loading — Ada

- **Ran:** 2026-08-06T02:11:27Z → 2026-08-06T02:14:55Z (box respected: yes)
- **Findings:** None. Every managed command exposed the intended supported path before resolution.
- **Bugs filed/updated:** None. The historical fixed hosted-MCP bug remains linked to the scenario.
- **Scenarios settled:** ET-managed-session-skill-loading → pass.
- **Paper cuts:** None.
- **Surprises:** None.
- **Suggested next charter:** Re-run the complete delayed-provider charter only when native-tool or provider behavior changes.

## What Was Fixed

No QA-discovered fixes were needed. This run verified the review-remediation working tree.

## Paper Cuts

None.

## Runtime Errors Observed

None. The CLI-only lab intentionally started no daemon or provider.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A copy-only guard change still needs executable public-interface proof because unit error identity
  checks do not prove the exact operator-facing sentence.
- The delayed provider and native-tool legs were unchanged and were not presented as freshly walked;
  their earlier full-walk evidence remains in
  `docs/qa/reports/2026-08-05-issue-314-managed-skill-loading.md`.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` — pass; current evidence is recorded in
  `.cache/gate/full.json`, and the exact log is indexed by the lab's `verify_gate` journey row.
- **Strict evidence audit:** pass — `/Users/pedronauck/dev/qa-labs/compozy-managed-skill-cli-guard-20260806-021127-858332-lab/qa-artifacts/qa/qa-audit-report.json`.
- **Process teardown:** clean — `/Users/pedronauck/dev/qa-labs/compozy-managed-skill-cli-guard-20260806-021127-858332-lab/qa-artifacts/qa/teardown.json` reports `"clean": true`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** all 12 managed `compozy skill` verbs, exact error copy, operator `skill view`, and
  operator `skill list`; unchanged delayed-provider/native-tool evidence carried forward explicitly.
- **Verdict:** ready — the changed public CLI boundary passed and no new issue was found.
