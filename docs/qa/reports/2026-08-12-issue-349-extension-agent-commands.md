# QA Run Report — 2026-08-12 — issue 349 extension agent commands

- **Scope:** Extension-published agent session commands, prompt expansion, public catalog parity, native command/skill reads, and workspace fencing
- **Cadence tier:** targeted
- **Build:** `f15d9e54` plus the PR #350 remediation working tree · **Environment:** fresh isolated targeted lab
- **Started:** 2026-08-12T00:13:41-03:00 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-session-command-catalog-parity |

## Flows in Scope

- `J-use-session-slash-commands` — discover and use session-effective commands without losing authored text (`../journeys/J-use-session-slash-commands.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-command-catalog-parity | J-use-session-slash-commands / ET-session-command-catalog-parity | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Bruno created an unbound session for the extension-published `reviewer`, inspected its command catalog before the first prompt, asked the live Codex agent to discover and open the exact extension skill through native tools, invoked the same skill as a slash command, and inspected the composer command menu before and after a browser refresh.

CLI, HTTP, and direct UDS returned 11 commands with revision `9fce06ed445f6e2e51a6110ca3dcc54c4e8e22ef7f9e8b031e8344b64cd74512`. Nine commands came from the global `dev-cycle` extension. The native tool results reported no error, the persisted slash invocation retained the source-qualified extension command id, and the foreign workspace request returned 404.

## What Was Fixed

The remediation keeps authored-agent validation first and falls back to the workspace-fenced resource catalog only when the authored registry reports `skills.ErrAgentNotFound`. It also makes native `skill_view(command_id)` depend explicitly on the same resolver and consolidates the regression coverage in the owning suites. The delivery commit is recorded on PR #350 after the completion gate.

## Paper Cuts

None.

## Runtime Errors Observed

None. Browser errors were empty; console output contained only Vite, React development, and expected session SSE lifecycle messages.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

An extension-published agent must retain a concrete source snapshot for exact skill resolution, while authored-agent validation remains authoritative. The resource catalog is a narrow fallback for agents absent from the authored registry, not a replacement for registry validation.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` (full `make verify`) — PASS; the current completion record is cited by `make gate-status`
- **Issues by user impact:** 0 open; GitHub #349 behavior passed
- **Coverage:** CLI, HTTP, direct UDS, Web refresh, native `command_list`, native source-qualified `skill_view`, plain prompt, slash-skill prompt, unbound runtime, and wrong-workspace fence
- **Evidence root:** `/Users/pedronauck/dev/qa-labs/compozy-issue-349-extension-agent-commands-20260812-031407-850286-lab/qa-artifacts/qa`
- **Verdict:** PASS
