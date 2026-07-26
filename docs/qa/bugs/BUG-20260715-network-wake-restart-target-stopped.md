# BUG-20260715-network-wake-restart-target-stopped: Queued Network wake fails after daemon restart

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-run-bounded-live-collaboration, restart before wake claim
- **Scenarios:** NB-run-bounded-live-collaboration
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

A durably queued Live wake survived daemon restart but failed immediately instead of resuming the persisted target session and prompting it. The source message remained durable, but its promised exactly-once activation could not complete.

## Reproduction

- **Charter:** CH-live-bounds-agent-path · **Tour:** Interrupt Tour
- **Environment:** desktop / isolated local daemon / en-US

1. Keep one Network wake in flight and admit a second durable wake for another Live owner.
2. Restart the daemon before the second wake is claimed.
3. Re-read the recovered wake, task run, provider diagnostics, conversation, and usage.

**Expected:** Recovery resumes the persisted target session, prompts once, and settles the original wake.
**Actual:** The first reproduction settled in 1 millisecond as `network wake prompt failed` because the recovered target session was stopped.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-live-bounds-agent-path.md`
- Deterministic provider trace: `qa/mock-recovery-worker-v2.jsonl` beside the lab manifest.

## Fix

- **Root cause:** The wake runner recovered the durable run but called `PromptNetwork` without first resuming its persisted target session; session prompting intentionally accepts active sessions only.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** `internal/daemon/network_wake_runner_test.go` proves startup recovery calls `Resume` exactly once before prompting the queued wake.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** `wake-aca546f93f266528` resumed `sess-f6e4d5140f5dc947`, prompted once, persisted one source message, and settled with actual usage.
