# BUG-20260807-gateway-provider-boot: Degraded provider prevents local daemon startup

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Dora
- **Journey Step:** J-audit-and-teardown-gateway, step 5
- **Scenarios:** RT-gateway-local-only-boot; RT-connectivity-provider-route; ET-connectivity-provider-trust
- **Found:** 2026-08-07 · **Report:** `docs/qa/reports/2026-08-07-remote-gateway.md`

## Summary

After Dora leaves a Tailscale provider enabled and restarts without its credential binding, the daemon exits during gateway reconciliation. The provider is already persisted as degraded and no endpoint is advertised, so the daemon should continue serving its local UDS and loopback control plane.

## Reproduction

- **Charter:** CH-gateway-audit-teardown · **Tour:** Feature Tour
- **Environment:** desktop / isolated lab / en-US

1. Enable the gateway ceiling and the bundled Tailscale provider.
2. Leave `TS_AUTHKEY` unbound so provider establishment fails closed.
3. Restart the daemon.

**Expected:** The daemon starts local-only, keeps the provider degraded, advertises no endpoint, and reports the actionable cause.
**Actual:** The daemon exits with `reconcile gateway before server advertisement`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/32-public-provider-degraded-status.json`
- Foreground restart at 2026-08-07T20:54:39Z exited before server advertisement with the missing-binding cause.

## Fix

- **Root cause:** The reconciler persisted the safe degraded state but returned only the raw provider error, so boot could not distinguish a compensated provider outage from a fatal persistence or composition failure.
- **Fix commit:** Task 09 QA remediation batch (pending local commit)
- **Regression test:** `TestGatewayBootContinuesLocalOnlyWhenProviderDegraded` plus the reconciler classification assertion in `TestPolicyReconcileCompensation`.

## Verification

- **Retested:** 2026-08-07
- **Result:** Pass — the daemon completed startup, retained the public provider as degraded, advertised no address, and served matching status through CLI, HTTP, and UDS.
- **Evidence:** `qa/test-cases/33-provider-restart-rewalk.json`
