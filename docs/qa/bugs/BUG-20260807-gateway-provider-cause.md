# BUG-20260807-gateway-provider-cause: Gateway hides the missing Tailscale credential

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-expose-and-pair-gateway, step 2
- **Scenarios:** RT-gateway-operator-surface-truth; RT-connectivity-provider-route; ET-connectivity-provider-trust
- **Found:** 2026-08-07 · **Report:** `docs/qa/reports/2026-08-07-remote-gateway.md`

## Summary

Dora enables the bundled Tailscale provider without a bound `TS_AUTHKEY`. The gateway correctly stays offline, but its durable status reports only `Internal error`, hiding the credential binding that the operator must repair.

## Reproduction

- **Charter:** CH-gateway-provider-degradation · **Tour:** Network Tour
- **Environment:** desktop / 1920×963 / isolated lab / en-US

1. Start a fresh daemon without `TS_AUTHKEY` in the authorized provider environment.
2. Enable the gateway ceiling.
3. Enable the bundled Tailscale provider for the public tier.
4. Enable the public webhook ingress surface.
5. Read the structured gateway status.

**Expected:** Exposure remains fail-closed and the status names the missing `TS_AUTHKEY` binding without exposing any credential value.
**Actual:** Exposure remains fail-closed, but the cause ends with `json-rpc error -32603: Internal error`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/31-public-surface-enable-provider-failure.json`
- `/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/32-public-provider-degraded-status.json`
- `/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/screenshots/05-provider-degraded-no-address.png`

## Fix

- **Root cause:** The provider returned an ordinary Go error. The SDK intentionally converted it to a generic internal JSON-RPC error, and the daemon intentionally exposed only the stable JSON-RPC message rather than untrusted error data.
- **Fix commit:** Task 09 QA remediation batch (pending local commit)
- **Regression test:** `TestBundledProviderRPCErrorContract`

## Verification

- **Retested:** 2026-08-07
- **Result:** Pass — CLI, HTTP, and UDS all report `TS_AUTHKEY binding is required`; no address or credential value is present.
- **Evidence:** `qa/test-cases/33-provider-restart-rewalk.json`
