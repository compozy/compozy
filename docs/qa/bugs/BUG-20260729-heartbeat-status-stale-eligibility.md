# BUG-20260729-heartbeat-status-stale-eligibility: Wake eligibility stayed stale in the mounted agent view

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-31, wait for the selected session to become Wake-eligible
- **Scenarios:** RT-077
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated browser replay

## Summary

The Heartbeat panel fetched selected-session status while the first prompt was active and kept showing
`session_prompt_active` after the daemon had transitioned the session to idle, healthy, and eligible.
Wake remained disabled until a full page reload.

## Reproduction

1. Open an agent's Heartbeat editor with an active Heartbeat policy.
2. Create a selected session with a first prompt and keep the agent view mounted.
3. Wait for the daemon session health to become `idle/healthy/eligible_for_wake=true`.

**Expected:** The mounted panel reconciles the selected session status and enables Wake.
**Actual before the fix:** The panel retained `session_prompt_active` beyond query freshness and enabled
Wake only after reload.

## Evidence

- Browser screenshots and live status assertions:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/040-agent-create-authored-files`.
- The first mounted replay remained disabled; the repaired replay enabled Wake after six seconds with
  no navigation or reload.

## Fix

- **Root cause:** the exact Heartbeat status query used a five-second freshness window but had no
  mounted-session reconciliation after its initial fetch.
- **Correction:** selected-session Heartbeat status now refetches every five seconds while mounted;
  policy-only reads remain event/refetch driven.
- **Fix commit:** pending completion gate
- **Regression test:** `Should refresh selected-session eligibility while heartbeat operations stay mounted`
  in `web/src/systems/agent/hooks/__tests__/use-agent-heartbeat.test.tsx`.

## Verification

- The regression failed red with one request where two were required, then passed after the query-owner
  correction.
- The live repaired SPA enabled Wake from the same mounted page after the session became eligible.
- `make lint` and `make build` pass.
- **Retested:** rebuilt web candidate green; governed fix commit pending
