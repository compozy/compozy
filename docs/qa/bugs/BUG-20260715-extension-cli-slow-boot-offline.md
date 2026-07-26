# BUG-20260715-extension-cli-slow-boot-offline: Extension CLI rejects a healthy slow-boot daemon as offline

- **Status:** verified
- **Impact (user-side):** Blocks extension and skill Marketplace operations
- **Severity:** High · **Priority:** P1
- **Personas Affected:** Ada; Vera
- **Journey Step:** J-extension-policy-admin, acquire or manage an extension
- **Scenarios:** ET-ext-curated-digest-verify; ET-cli-extension-sideload-policy-block
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

The daemon metadata timestamp is written after early boot work. A real startup took about five seconds before writing `started_at`, but the shared process-identity heuristic allowed only two seconds of divergence. Extension and skill commands treated that healthy daemon as offline even though its UDS status endpoint was reachable and reported `running`.

## Reproduction

1. Start the daemon with enough configured resources that early boot exceeds the process timestamp tolerance.
2. Confirm the daemon PID is alive and `agh status` succeeds over UDS.
3. Run an extension Marketplace install or another daemon-owned extension operation.

**Expected:** A reachable UDS status of `running` is authoritative when the PID is alive; an unreachable or dead process remains offline.
**Actual:** The CLI returned `extension marketplace operations require a running daemon` without probing the healthy transport.

## Evidence

- Sanitized red/green and live replay: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/extension-cli-slow-boot-reachability.json`.
- The current isolated daemon started at 23:20:45 local and wrote logical `started_at` 5.538 seconds later, still exceeding the removed two-second decision boundary.

## Fix

- **Root cause:** Extension/skill client selection treated a process-start timestamp mismatch as definitive offline state, unlike the status command's transport-first behavior.
- **Correction:** If the recorded PID is alive but its timestamp diverges, client selection probes daemon status and accepts only a structured `running` response. A dead PID, unavailable transport, or non-running status remains offline; non-availability errors are propagated.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** The canonical extension Marketplace suite forces timestamp mismatch with a live PID and requires install to reach the daemon. Its existing offline case still requires deterministic rejection.

## Verification

- The regression failed before the reachability fallback and passes afterward.
- 121 focused extension and skill CLI cases pass under `-race`.
- On the rebuilt daemon whose metadata lag is 5.538 seconds, `agh extension search qa -o json` reached the daemon and returned `acme/qa-marketplace-extension` instead of false offline state.
