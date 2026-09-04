# QA Run Report — 2026-09-04 — Provider switch runtime settings

- **Scope:** PR #546 provider identity handling in `compozy agent update`.
- **Cadence tier:** targeted
- **Build:** working tree based on `897e377b8d0db462541a3f14b987ac223f818464` · **Environment:** isolated CLI-only runtime lab `pr-546-provider-switch-20260904-152325-974742`
- **Started:** 2026-09-04T15:27:52Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-provider-switch-settings |

## Flows in Scope

- `J-32` — Manage an agent definition through structured CLI output and confirm independently read durable state (`../journeys/J-32-manage-agent-lifecycle.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-provider-switch-settings | J-32 / RT-081 | Ada | Feature Tour | Pass | | this remediation commit |

## Session Debriefs

### CH-provider-switch-settings — Ada

- **Ran:** 2026-09-04T15:29:36Z → 2026-09-04T15:32:04Z (box respected: yes)
- **Findings:** None. The alias/case-only update retained every provider-owned runtime setting, while the real provider change cleared all four omitted settings.
- **Bugs filed/updated:** None.
- **Scenarios settled:** RT-081 → pass.
- **Paper cuts:** None.
- **Surprises:** The authored provider spelling remains exactly as requested while effective runtime identity is canonicalized, making structured output both faithful and safe.
- **Suggested next charter:** Re-run the broader existing CH-052 lifecycle parity charter in the next full J-32 cycle.

## What Was Fixed

### Equivalent provider names no longer clear runtime settings

- **Symptom:** Updating `provider` from a built-in alias or case variant to the same provider identity silently cleared `command`, `model`, `reasoning_effort`, and `acp_options` when the flags were omitted.
- **Root cause:** The CLI request builder compared authored provider strings directly instead of comparing canonical provider identities.
- **Fix:** Compare canonical identities only for sanitization while retaining the requested provider spelling in the update payload.
- **Regression test:** `internal/cli/agent_commands_test.go` — the alias and case table failed before the fix and passes after it under `go test -race`.
- **Retested:** J-32 / RT-081 through create → equivalent update → independent read → real provider update → independent read.

## Paper Cuts

None observed.

## Runtime Errors Observed

The daemon rejected a setup attempt that combined the `kimi` alias with unsupported reasoning effort. This was expected provider-contract validation; the successful walk used Claude's supported reasoning contract.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Provider identity and authored provider spelling are separate contracts: canonical identity governs sanitization, while structured output preserves what the caller requested.

## Final Status

- **Exit evidence:** `CGO_ENABLED=1 go test -race ./internal/cli ./internal/config -count=1` passed; targeted structured CLI walk passed. Per the controller correction, no local `make gate` or global gate was run; GitHub CI owns comprehensive validation.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-pr-546-provider-switch-20260904-152325-974742-lab/qa-artifacts/qa/provider-switch-evidence.md`
- **QA audit:** Functional evidence checks passed, but strict audit C14 remains blocked because it requires a local gate that the controller explicitly prohibited. Report: `/Users/pedronauck/dev/qa-labs/compozy-pr-546-provider-switch-20260904-152325-974742-lab/qa-artifacts/qa/qa-audit-report.json`.
- **Teardown:** Clean with zero survivors at `/Users/pedronauck/dev/qa-labs/compozy-pr-546-provider-switch-20260904-152325-974742-lab/qa-artifacts/qa/teardown.json`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 targeted journey walked; broader create/duplicate/delete/session behavior remained outside this narrow PR scope.
- **Verdict:** PASS — the corrected provider-switch behavior is ready for focused repository checks and exact-head PR CI.
