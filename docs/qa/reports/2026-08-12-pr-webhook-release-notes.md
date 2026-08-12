# QA Run Report — 2026-08-12 — PR webhook release notes

- **Scope:** Operator-home/global Gateway config collision plus the CompozyOS webhook, Tailscale Funnel, Agent, and `releasepr` dogfood path
- **Cadence tier:** targeted
- **Build:** `714b7347` working tree · **Environment:** primary operator daemon, real Tailscale Funnel, real GitHub PR #358
- **Started:** 2026-08-12T17:06:38Z · **Status:** blocked

The report was materialized immediately after the first live config checkpoint. The command evidence
and daemon records below preserve that checkpoint; no session was reconstructed from mocks.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Runtime Administrator | desktop / wifi-fast / en-US | CH-gateway-global-workspace-config |

## Flows in Scope

- `J-expose-and-pair-gateway` — enable the global Gateway ceiling without losing local readiness or workspace registration (`../journeys/J-expose-and-pair-gateway.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-gateway-global-workspace-config | J-expose-and-pair-gateway / MS-gateway-config-ceiling | Dora | Feature Tour | Fixed | BUG-20260812-global-workspace-gateway-config | working tree |
| 2 | CH-gateway-global-workspace-config | J-expose-and-pair-gateway / RT-gateway-local-only-boot | Dora | Feature Tour | Fixed | BUG-20260812-global-workspace-gateway-config | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-gateway-global-workspace-config — Dora

- **Ran:** 2026-08-12T17:00:52Z → 2026-08-12T17:06:55Z (box respected: yes)
- **Findings:**
  - The primary daemon started with `/Users/pedronauck/.compozy/config.toml`, while both `/Users/pedronauck` and the Compozy repository remained registered workspaces.
  - `compozy config set gateway.enabled true --scope global -o json` reported `lifecycle=live`, `applied=true`, and `restart_required=false` without the prior global-only error.
  - Structured status independently reported the daemon running, config validation current, Gateway enabled, and later the public Tailscale provider and `webhook_ingress` surface healthy.
- **Bugs filed/updated:** BUG-20260812-global-workspace-gateway-config
- **Scenarios settled:** MS-gateway-config-ceiling → pass; RT-gateway-local-only-boot → pass
- **Paper cuts:** none
- **Surprises:** Tailscale public DNS initially returned NXDOMAIN, then the existing Gateway recovery loop advertised the endpoint without manual teardown or a second home.
- **Suggested next charter:** Re-run the same config walk after an operator-home path case-variation on a case-insensitive filesystem.

Edge probes attempted and clean: repeat the same live write; read status from the repository cwd;
retain both registered workspaces; wait through transient public DNS failure; reject an unsigned public
webhook. The external signed webhook then created exactly one automation run. The first exact-head
check returned `stale_head` after the PR advanced. An earlier exact-head run for PR #358 completed
as `fork_read_only`; later manual deliveries were canceled when the test client closed its request
after 120 seconds. The GitHub workflow now allows 840 seconds for the synchronous Agent run.

Experiential lenses: usability pass (structured next state), accessibility not applicable to the CLI
walk, perceived performance pass aside from the documented Tailscale DNS propagation window,
compatibility pass on the operator macOS host, error recovery pass through automatic Gateway retry,
and production parity pass with the primary daemon and real external services.

## What Was Fixed

### BUG-20260812-global-workspace-gateway-config: Global Gateway config blocks the operator-home workspace

- **Symptom:** a valid global `[gateway]` section prevented startup or live config writes when the operator home was also a workspace.
- **Root cause:** the same canonical config file was applied twice under conflicting global and workspace semantics.
- **Fix:** skip the workspace overlay only when its canonical path equals the global config path; distinct workspace overlays retain the global-only rejection.
- **Regression test:** `internal/config/gateway_test.go` and `internal/config/persistence_integration_test.go` failed before the fix and pass after it.
- **Retested:** J-expose-and-pair-gateway plus the public Funnel/webhook integration canary on the primary daemon.

## Paper Cuts

None observed.

## Runtime Errors Observed

- Tailscale public DNS returned NXDOMAIN during initial Funnel propagation. Gateway retained the provider session and recovered automatically when the public record appeared; no product bug was filed.
- Two manual webhook deliveries ended with `context canceled` when the sending client reached its 120-second deadline. The production GitHub workflow uses a 15-minute job and an 840-second request deadline so the request context remains alive for the Agent run.

## Human Verifications Needed

The repository-wide close gate is blocked by a pre-existing formatting issue in `skills-lock.json`,
which is outside this workstream and was left untouched.

## Decisions for a Human

None.

## Learnings

- A single physical config file must never be reinterpreted at two scopes.
- Tailscale Funnel DNS propagation is transient; Gateway recovery, not another runtime home, owns that wait.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` — blocked in Bun lint by the unrelated, pre-existing `skills-lock.json` formatting issue. The Agent artifact created by this workstream is formatted.
- **Issues by user impact:** Blocks-Completion 0 open · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1 journey walked / 1 in scope; no skips
- **Verdict:** integration ready; repository close blocked by the unrelated formatting issue above. The global config collision is fixed and the real signed webhook path reaches the workspace Agent through Tailscale Funnel.
