# BUG-20260826-parked-child-idle-clock: Parked child idle clocks disappeared after settlement

- **Status:** fixed and publicly verified
- **Impact (user-side):** Breaks-Flow
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-message-a-running-agent — park, inspect, and expire a completed call child
- **Scenarios:** RT-parked-child-idle-ttl
- **Found:** 2026-08-26 · **Report:** `docs/qa/reports/2026-08-26-agent-comms.md`

## Summary

A child settled a call with a short idle TTL, but stopping that same child canceled the return
request context. The compensation path then cleared the newly persisted idle clock. After the
clock was preserved, public call reads still projected the stopped session from stale in-memory
metadata and reported `idle_expires_at: null` instead of the durable catalog value.

## Reproduction

1. Create a strict call with `--idle-ttl 2s` and let the child return a valid result.
2. Await the completed call and inspect it through `compozy call show -o json`.
3. Wait beyond the requested TTL and call the child session id again.

**Expected:** The completed record exposes a concrete `idle_expires_at`, and a follow-up after that
instant fails with `call_target_expired`.
**Actual:** The public record kept `idle_expires_at: null`, and the child accepted follow-ups after
the requested TTL.

## Fix

- **Root cause:** The child stop inherited the tool request cancellation boundary, and public
  status reads preferred stopped in-memory session metadata over catalog-owned parking fields.
- **Production fix:** Terminal settlement stops the child under a bounded detached context and uses
  a detached compensation context if that stop fails. Stopped spawned-session status overlays
  `parked_at` and `idle_expires_at` from the durable session catalog.
- **Regression:** The canonical call settlement suite proves the idle clock survives cancellation
  of the return request. The canonical session query suite proves stopped children project the
  durable parking lifecycle.
- **Fix commits:** `ed9c5ac`; `8181f49`.

## Verification

- `go test -race ./internal/calls -count=1` — 84 tests passed.
- `go test -race ./internal/session -count=1` — 1,014 tests passed.
- Public retest call `call-6b13897bb94361c8` exposed
  `idle_expires_at: 2026-08-26T08:57:55.04644Z`; its child
  `ses_call_call-6b13897bb94361c8` then rejected a follow-up with
  `call_target_expired` naming the same timestamp and suggesting a fresh agent call.
- Evidence: `/Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/ttl-public-retest.md`.
