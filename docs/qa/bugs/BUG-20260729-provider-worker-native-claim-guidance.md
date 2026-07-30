# BUG-20260729-provider-worker-native-claim-guidance: Provider workers cannot claim scheduler runs through prescribed CLI path

- **Status:** verified
- **Impact (user-side):** Blocks autonomous task execution
- **Severity:** Critical · **Priority:** P0
- **Personas Affected:** Bruno; Priya
- **Journey Step:** consumer-saas-growth, autonomous worker execution after operator kickoff
- **Scenarios:** TA-task-role-session-activation; TA-exact-claim-single-owner
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-29-ext-improvs.md
- **Origin:** Task 11 isolated consumer SaaS growth replay

## Summary

The scheduler delivered queued runs to healthy Codex sessions, but its synthetic reentry and the official Compozy skill directed workers to claim with `compozy task next`. The provider shell intentionally lacked usable daemon and session identity, so every CLI claim failed while the session-bound native claim tool remained available. Ten runs exhausted the escalation ladder and entered `needs_attention` without doing their work.

## Reproduction

1. Start an isolated daemon with Codex-backed workspace agents and hosted native tools.
2. Enqueue workspace task runs, resume the scheduler, and let it wake eligible active sessions.
3. Follow the scheduler message and official worker-loop guidance by invoking `compozy task next --run-id <run-id> -o json` from the provider shell.

**Expected:** The wake directs the worker to the hosted session-bound claim surface, and the worker claims the assigned run with daemon-supplied session/workspace identity.
**Actual:** The prescribed CLI path returns `identity_required` or `identity_lookup_unavailable`; the run remains queued until the scheduler parks it in `needs_attention`.

## Evidence

- Failed lab: `/Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260729-211028-982531-lab`.
- Frontend worker session `sess-be9d820ee5a7d82a`, run `run-90ce600c1cd01bcd`, records the synthetic wake, repeated CLI claim attempts, and identity failures.
- Growth PM session `sess-1fe6ba2a245686e5` successfully invoked `mcp.compozy-hosted-tools.compozy__task_run_claim_next` in the same lab, proving native availability.
- The failed lab ended with `needs_attention_run_count: 10`; teardown evidence reports `TEARDOWN_ALL_CLEAN=true`.

## Fix

- **Root cause:** Daemon-owned wake prompts and the official worker-loop reference treated a provider shell CLI subprocess as equivalent to the hosted native tool, even though only the hosted tool carries trusted caller session and workspace scope.
- **Correction:** Scheduler, coordinator, and task-role wake prompts now name the hosted `compozy__task_run_claim_next` lease tool with the assigned run id. The official Compozy worker-loop reference uses the same native contract and no longer prescribes CLI self-claim from provider shells.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** Canonical coordinator, scheduler-heartbeat, task-role, and session-policy suites require `compozy__task_run_claim_next` with the assigned run id and reject CLI claim guidance.

## Verification

- Focused coordinator and daemon suites pass under `-race`; the complete daemon package passes under `-race`.
- In the fresh isolated replay, seven Codex workers claimed through the hosted native lease tool, all eleven declared runs completed, and no run entered `needs_attention`.
