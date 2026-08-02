# QA Run Report — 2026-08-01 — loops-paper Task 01 MCP manual timeout

- **Scope:** Branch-targeted replay of pending stdin cancellation for `compozy mcp auth login <name> --manual`; adjacent CLI/API status reads are the canary.
- **Cadence tier:** targeted
- **Build:** `599b32dd` + Task 01 working tree · **Environment:** fresh isolated local daemon and a protocol-conformant deterministic OAuth authority over public HTTPS
- **Started:** 2026-08-01T08:08:00Z · **Ended:** 2026-08-01T08:26:19Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Iris | Remote operator | terminal / loopback / en-US | CH-remote-operator-manual-auth |

## Flows in Scope

- `J-mcp-authorize-repair` — begin manual OAuth, wait for pasted input, and leave credential state truthful on timeout (`../journeys/J-mcp-authorize-repair.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-remote-operator-manual-auth | J-mcp-authorize-repair / ET-cli-mcp-auth-manual-exchange | Iris | Paste Tour | Fixed | BUG-20260801-mcp-manual-input-timeout | 38b2d40 |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Iris began manual authorization against a real daemon and an OAuth authority reachable through
public HTTPS, then left inherited stdin open without sending data. The first replay revealed that
an inherited FIFO did not support direct Go file deadlines. After the root correction, the same
replay exited in 0.966 seconds with the canonical timeout. CLI and HTTP status remained
`needs_login` with `token_present:false`; the provider saw no token exchange.

## What Was Fixed

- Inherited non-terminal stdin now participates in deadline cancellation without closing or leaving
  the caller-owned descriptor nonblocking.
- Expected poller timeout details no longer leak beneath the stable CLI authorization-timeout error.

## Paper Cuts

None.

## Runtime Errors Observed

The initial loopback-provider attempt was rejected by the daemon's SSRF policy before stdin. That
was an expected security boundary, not a product failure; the authority was exposed through a
temporary public HTTPS tunnel for the valid replay. The first public replay then found the in-scope
inherited-stdin deadline defect recorded above.

## Human Verifications Needed

None identified for the deterministic pending-input leg.

## Decisions for a Human

None.

## Learnings

`os.Pipe` created inside a Go process is not equivalent to stdin inherited across `exec`: inherited
blocking descriptors are not registered with Go's poller. Deadline tests for CLI stdin must model
the inherited descriptor and verify both cancellation and restoration of borrowed file flags.

## Final Status

- **Exit gate (full automated suite):** task closure uses the post-report `make gate` evidence record.
- **Issues by user impact:** Blocks-Completion 1 fixed and retested · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 rows terminal
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loops-paper-task01-mcp-manual-timeout-20260801-075922-632496-lab/qa-artifacts/qa/notes/manual-input-timeout-public.json`
- **Audit scope:** targeted `qa-execution` replay only; no feature-grade multi-agent or web scenario is claimed.
- **Verdict:** pass — the public pending-input leg is fixed, retested, unauthenticated after timeout, and free of submitted OAuth material.
