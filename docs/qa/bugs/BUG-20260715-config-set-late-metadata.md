# BUG-20260715-config-set-late-metadata: Live config writes bypass a reachable slow-boot daemon

- **Status:** verified
- **Impact (user-side):** Reports false success while the runtime keeps the previous configuration
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Vera
- **Journey Step:** J-extension-policy-admin, switch the curated feed without a restart
- **Scenarios:** MS-marketplace-catalog-live-config; ET-marketplace-kill-switch
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

`agh config set` used daemon process metadata as the only reachability decision for ordinary global writes. When early boot made the recorded daemon timestamp diverge from the process timestamp, a live daemon was treated as offline. The CLI wrote `config.toml` locally and reported `applied=true`, but emitted no apply record, did not advance the active generation, and left the Marketplace runtime on the previous feed.

## Reproduction

1. Start a daemon whose early boot exceeds the process timestamp tolerance while its UDS status endpoint is healthy.
2. Run `agh config set marketplace.catalog.base_url <second-feed> -o json`.
3. Run `agh marketplace refresh --kind mcp -o json` and inspect apply history.

**Expected:** A live PID with a reachable UDS `running` status is reconciled through the daemon. The result includes an apply record and active generation, and the next refresh uses the second feed.
**Actual:** The CLI returned `lifecycle=live` and `applied=true` without an apply record. Apply history did not advance, and refresh continued to use the first feed.

## Evidence

- Sanitized red/green replay: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-config-reachability.json`.
- The canonical CLI regression failed before the shared reachability owner was used: `ReloadSettings was not called through the reachable daemon`.

## Fix

- **Root cause:** The robust PID-plus-UDS fallback was extension-specific. Generic config mutation still trusted process timestamp metadata as a definitive offline signal, and its local lifecycle classifier could claim a live value was applied without daemon confirmation.
- **Correction:** Daemon reachability is now owned by one generic CLI helper. Extension, skill, and config flows all probe structured UDS status when the PID is live but timestamp metadata lags. Global config writes invoke `ReloadSettings` and project only the daemon-confirmed lifecycle.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** The canonical config command suite forces a live PID with timestamp mismatch, requires `ReloadSettings`, and asserts the returned apply record and active generation.

## Verification

- `go test -race ./internal/cli -count=1` passed.
- Live switch to the second feed returned apply record `cfgapp-2497d53aaa5fa1bd`, advanced active generation 6 → 7, and projected only `qa-secondary-mcp`.
- Restoring the primary feed returned apply record `cfgapp-9884f9162558b007`, advanced generation 7 → 8, and restored exactly the two primary MCP entries.
