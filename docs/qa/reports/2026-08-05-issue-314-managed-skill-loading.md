# QA Run Report — 2026-08-05 — Issue 314 managed skill loading

- **Scope:** Issue #314 — managed Codex sessions load installed skills through the native seam or documented CLI fallback while preserving identity and policy boundaries
- **Cadence tier:** targeted
- **Build:** working tree · **Environment:** isolated local daemon and provider home from the QA bootstrap manifest
- **Started:** 2026-08-05T18:09:45-03:00 · **Completed:** 2026-08-05T18:24:14-03:00 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-managed-session-skill-loading |

## Flows in Scope

- `J-load-skill-in-managed-session` — load one installed skill through native and CLI paths without gaining broader authority (`../journeys/J-load-skill-in-managed-session.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-managed-session-skill-loading | J-load-skill-in-managed-session / ET-managed-session-skill-loading | Ada | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-managed-session-skill-loading

- Session `sess-fde197161ff83533` ran with workspace `ws_8d47ffbddfb5e2ef`, agent `general`, Codex `gpt-5.6-sol`, and workspace permission mode `approve-reads`.
- Native `compozy__skill_view` returned the workspace `release-signal` body with `HELIX-SKILL-314` and the 15% threshold (event 32).
- The provider process exposed `COMPOZY_AGENT_TRANSPORT_SOCKET=/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/cz-agent-1284359853/transport.sock`; `test -S` returned 0 (event 59).
- Managed `compozy skill view release-signal` returned the same body with exit 0 (event 72). Replacing only `COMPOZY_SESSION_ID` and `COMPOZY_AGENT` still returned the bound session body with exit 0 (event 80).
- `--for-agent reviewer` was denied with exit 77 (event 88). Foreign workspace `ws_2f579f45788c8c97` was denied by the session permission policy with exit 77 (event 98).
- `compozy status` was rejected by the capability transport with `managed_transport_denied` and exit 1 (event 106). A final own-scope skill read still succeeded with exit 0 (event 109).
- An operator shell with all managed identity and transport variables unset read the same body through the regular daemon socket with exit 0.
- Stopping the session removed both the capability socket and its `cz-agent-1284359853` directory. The mandatory teardown stopped registered daemon PID 13473 and recorded `clean: true` with no survivors.

### Read-only capability retest

- Session `sess-518717aea033acef` ran in workspace `ws_5dbcd88c162fa761` with the final read-only route allowlist.
- Managed `compozy skill view release-signal` returned the distinctive body with exit 0 (event 50).
- Managed `compozy skill disable release-signal` was rejected with `managed_transport_denied` and exit 1 (event 58).
- A second `compozy skill view release-signal` returned the same body with exit 0 (event 64), proving the denied mutation did not damage the read path.
- The mandatory teardown stopped registered daemon PID 27403 and recorded `clean: true` with no survivors in `/Users/pedronauck/dev/qa-labs/compozy-issue-314-read-only-skill-transport-20260805-211857-719811-lab/qa-artifacts/qa/teardown.json`.

## What Was Fixed

The managed CLI fallback now enters through a per-session capability socket owned by the local ACP process. That socket only proxies read-only skill routes and replaces caller-supplied identity headers with daemon-owned session and agent identity.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

- Expected denial: mismatched agent scope, exit 77.
- Expected denial: foreign workspace under `approve-reads`, exit 77.
- Expected denial: non-skill `compozy status` route, exit 1 with `managed_transport_denied`.
- Expected denial: skill mutation through the managed read-only transport, exit 1 with `managed_transport_denied`.

## Human Verifications Needed

None recorded yet.

## Decisions for a Human

None recorded yet.

## Learnings

The capability must be attached to the ACP provider process, not an individual terminal callback. Codex executes internal commands as descendants of `codex app-server`, so a terminal-only descriptor does not reach those commands. Process-scoped socket ownership also gives one lifecycle boundary for provider commands and terminal callbacks.

## Final Status

- **Exit gate (full automated suite):** recorded by `make gate-status` at workstream close
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** native tool, managed CLI, forged environment identity, mismatched agent, foreign workspace policy, non-skill route denial, skill-mutation denial, operator CLI, final own-scope recovery, and process teardown
- **Verdict:** Pass
