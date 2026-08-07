# BUG-20260807-gateway-native-tool-wiring: Native gateway tool panics after daemon boot

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Dora
- **Journey Step:** J-audit-and-teardown-gateway, step 2
- **Scenarios:** RT-gateway-self-audit
- **Found:** 2026-08-07 · **Report:** `docs/qa/reports/2026-08-07-remote-gateway.md`

## Summary

`compozy tool invoke compozy__gateway --input '{"action":"audit"}'` returns HTTP 500 while the same audit succeeds through CLI, HTTP, and UDS. The daemon recovers a nil-pointer panic from the native invocation.

## Reproduction

- **Charter:** CH-gateway-audit-teardown · **Tour:** Feature Tour
- **Environment:** CLI / isolated lab / en-US

1. Start the daemon in local-only mode.
2. Run the gateway audit through CLI, HTTP, and UDS; all return `no_findings`.
3. Invoke the `compozy__gateway` native tool with `{"action":"audit"}`.

**Expected:** All four surfaces return the same redacted local-only audit.
**Actual:** The native tool returns HTTP 500 and panics at `gateway.Service.Status` because its policy is nil.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/38-native-audit-parity.json`
- Daemon panic captured during the 2026-08-07 Task 09 run.

## Fix

- **Root cause:** The native tool registry boots before the gateway service. Converting the then-nil `*gateway.Service` to the native dependency interface produced a non-nil typed interface that remained stale after gateway boot.
- **Fix commit:** Task 09 QA remediation batch (pending local commit)
- **Regression test:** `TestDaemonNativeGatewayTool/Should_resolve_the_gateway_service_lazily_after_registry_construction`

## Verification

- **Retested:** 2026-08-07
- **Result:** Pass — CLI, HTTP, UDS, and `compozy__gateway` returned byte-equivalent canonical audit payloads; the native result was completed and redacted.
