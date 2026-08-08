# BUG-20260808-bridge-enable-without-config: A bridge without optional provider configuration cannot start

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Maya
- **Journey Step:** J-watch-agent-work-channel, entry and step 1
- **Scenarios:** NB-bridge-tool-progress
- **Found:** 2026-08-08 · **Report:** docs/qa/reports/2026-08-08-remote-gateway-toolmeta-remediation.md

## Summary

Maya could not start an otherwise valid bridge when its optional provider configuration was absent, so no bridged turn or progress lifecycle could begin.

## Reproduction

- **Charter:** CH-bridge-progress-stress · **Tour:** Garbage Tour
- **Environment:** laptop / wifi-slow / en-US; fresh isolated targeted runtime

1. Create a bridge whose provider does not require `provider_config`.
2. Enable the bridge through the public bridge API.

**Expected:** The bridge starts and treats the missing local callback configuration as unavailable until configured.
**Actual:** Enable returned HTTP 500 because the store tried to scan SQL `NULL` into a Go string.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-toolmeta-remediation-20260808-060444-758800-lab/qa-artifacts/qa/test-cases/10-bridge-progress-integration.log`
- `internal/store/globaldb/global_db_gateway_test.go` reproduced the same scan failure against real SQLite before the fix.

## Fix

- **Root cause:** `GetGatewayBridgeIngressSubject` combined two nullable `provider_config` sources while sqlc generated a non-nullable string result. SQLite returned `NULL`, so resolution failed before the domain could classify the target as unavailable.
- **Fix commit:** final remote-gateway review-remediation batch pending
- **Regression test:** `internal/store/globaldb/global_db_gateway_test.go` — `TestGlobalDBGatewayIngressLifecycle/Should treat a missing bridge provider config as an unavailable local target`

## Verification

- **Retested:** 2026-08-08, same persona/journey · **Report:** docs/qa/reports/2026-08-08-remote-gateway-toolmeta-remediation.md
- **Result:** Both canonical-resource and materialized bridge rows without provider configuration resolve successfully, and the full bridge integration harness reaches its terminal state under `-race`.
