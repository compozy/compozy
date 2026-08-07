# BUG-20260807-gateway-live-config-copy: Gateway settings incorrectly require a restart

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-expose-and-pair-gateway, step 1
- **Scenarios:** RT-gateway-operator-surface-truth; MS-gateway-config-ceiling
- **Found:** 2026-08-07 · **Report:** `docs/qa/reports/2026-08-07-remote-gateway.md`

## Summary

Dora sees an instruction to restart the daemon after enabling the gateway ceiling even though the change applies immediately. Following the instruction would interrupt work unnecessarily and makes the Settings page disagree with the runtime.

## Reproduction

- **Charter:** CH-gateway-provider-degradation · **Tour:** Network Tour
- **Environment:** desktop / 1920×963 / local isolated lab / en-US

1. Start a fresh daemon with `gateway.enabled=false`.
2. Open Settings → Gateway.
3. Read the warning under Reachability.
4. Run `compozy config set gateway.enabled true -o json` without restarting.

**Expected:** Settings says the key applies immediately, matching the config lifecycle result.
**Actual:** Settings says to restart the daemon, while the CLI returns `lifecycle: live`, `applied: true`, and `restart_required: false`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/screenshots/02-gateway-local-only-ready.png`
- `/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/07-live-enable-config-set.json`

## Fix

- **Root cause:** The ceiling warning contained stale static copy from before `gateway.enabled` was classified as a live config key.
- **Fix commit:** Task 09 QA remediation batch (pending local commit)
- **Regression test:** documented browser replay plus the existing config lifecycle contract; a component prose assertion would duplicate the owning runtime invariant.

## Verification

- **Retested:** 2026-08-07
- **Result:** Pass — the Settings warning now says the key applies immediately; toggling the ceiling through the CLI changed runtime state without a daemon restart.
- **Evidence:** `qa/screenshots/03-gateway-live-copy-fixed.png`, `04-gateway-enabled-live.png`
