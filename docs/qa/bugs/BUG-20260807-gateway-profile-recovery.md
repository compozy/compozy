# BUG-20260807-gateway-profile-recovery: Unreachable pairing blocks every profile command

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Iris
- **Journey Step:** J-operate-remote-gateway-cli, step 2
- **Scenarios:** RT-gateway-remote-cli-profile
- **Found:** 2026-08-07 · **Report:** `docs/qa/reports/2026-08-07-remote-gateway.md`

## Summary

Pairing with an address that refuses the TCP connection returns the correct reachability error, but leaves the prepared credential and recovery journal behind. Every later profile command then returns `gateway_pairing_recovery_pending`, even after the five-minute pairing window.

## Reproduction

- **Charter:** CH-gateway-remote-cli-interruption · **Tour:** Interrupt Tour
- **Environment:** CLI / isolated lab / en-US

1. Mint a pairing artifact locally.
2. Run `compozy connect add` against an unused HTTPS port.
3. Run `compozy connect list` after the pairing window.

**Expected:** The first command returns `gateway_reachability_failed` and atomically removes the unused credential, profile metadata, and transaction journal.
**Actual:** The first command returns the reachability error, but later profile commands remain blocked by `gateway_pairing_recovery_pending`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/35-remote-cli-interruption.json`

## Fix

- **Root cause:** The recovery design preserved every non-HTTP pairing error as an uncertain remote outcome. A TCP dial failure is not uncertain: no request reached the gateway, so retaining the local transaction cannot protect a remote device.
- **Fix commit:** Task 09 QA remediation batch (pending local commit)
- **Regression test:** `TestGatewayProfileTransactions/Should_roll_back_pairing_when_the_request_never_reached_the_gateway_US-019.EC-1`

## Verification

- **Retested:** 2026-08-07
- **Result:** Pass — the live CLI returned `gateway_reachability_failed`, then `connect list` stayed usable with zero profiles, zero candidate credential files, and zero pairing journals.
