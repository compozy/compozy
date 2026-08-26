# BUG-20260826-bounded-wait-client-timeout: The CLI transport interrupted valid waits after 30 seconds

- **Status:** fixed and publicly verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada; Bruno
- **Journey Step:** J-delegate-work-to-an-agent — await a call checkpoint; J-15 — wait for a session state
- **Scenarios:** RT-agent-call-deadline-timeout; RT-session-wait-state
- **Found:** 2026-08-26 · **Report:** `docs/qa/reports/2026-08-26-agent-comms.md`

## Summary

`compozy call await` and `compozy session wait` accept bounded waits up to 30 minutes, but both used
the CLI's generic HTTP client with a fixed 30-second timeout. Any valid wait longer than that failed
as a transport error before the daemon could report a checkpoint or state transition.

## Reproduction

1. Start a call or session that will not reach the requested state within 30 seconds.
2. Run `call await --timeout 60s` or `session wait --timeout 55s` over local UDS.
3. Observe the client error around 30 seconds.

**Expected:** The daemon owns the bounded wait and returns its normal timeout/state outcome.
**Actual:** The client returned `Client.Timeout exceeded while awaiting headers` first.

## Fix

- **Root cause:** Both bounded-wait client methods used the normal JSON transport whose client-level
  timeout is 30 seconds, independent of the public request's `timeout_ms`.
- **Production fix:** Bounded waits now use the dedicated no-global-timeout transport with an
  explicit context deadline equal to the requested wait, clamped to the public 30-minute ceiling,
  plus five seconds for response delivery. An earlier caller context deadline still wins.
- **Regression:** The canonical CLI client suite proves call and session waits use the dedicated
  transport, carry the requested deadline beyond 30 seconds, and clamp oversized waits.
- **Fix commit:** `cf46ed340`.

## Verification

- Focused regression: `go test -race ./internal/cli -run 'TestBoundedWaitClientsUseRequestedDeadline|TestNewClientConfiguresTimeouts' -count=1` — 5 tests passed.
- Public UDS retest: call await and session wait both returned their normal timeout checkpoints after
  35 seconds (35.29s and 35.07s), beyond the former 30-second transport deadline.
- Retest evidence: `/Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/qa-remediation-public-retest.md`.
- Reproduction evidence: `/Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/bounded-wait-client-timeout-reproduction.md`.
