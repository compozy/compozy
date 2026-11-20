---
id: RT-always-on-daily-spend
area: RT
title: Daily spend guardrails hold the next unattended instance turn
persona: Operator
journey: J-bound-runaway-work
expected: An optional token or estimated-USD limit per always-on instance counts each durable Done receipt once; 80% emits one bell per local day; reaching a cap holds the next autonomous turn with a persistent reason and requires eligible operator resume, without blocking human work or other profiles.
entry_points: compozy agent instance budget; compozy agent instance show; compozy agent instance resume; PATCH /api/agent-instances/{id}; GET /api/agent-instances/{id}; always-on autonomous prompt; instance attention bell
qa_status: needs-visual-validation
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/daemon_daily_budget_integration_test.go (TestDaemonDailyBudgetRealDriverJourney), internal/store/globaldb/global_db_daily_budget_backfill_receipts_test.go, internal/store/globaldb/global_db_daily_budget_backfill_retention_test.go
last_report:
overlaps: MS-always-on-check-in-sources; RT-spawn-budget-context
---

1. In an isolated home, create an always-on instance in profile A, configure a
   100-token daily limit and leave a peer in profile B unrestricted.
2. Record one 85-token completed turn using the controlled ACP provider.
   Confirm one bell for A, exact 85-token status, no bell for B.
3. Complete another 20-token turn already admitted; verify the subtotal is 105
   (in-flight overshoot) and replay the same Done. No duplicate receipt or bell.
4. Attempt the next automatic prompt or managed Goal claim. Verify it does not
   reach the driver, the pause reason is `daily_budget`, the queued input survives,
   and authenticated operator prompts remain claimable even behind a prepared
   Goal during a budget pause or a latched recovery hold. Submit app-origin
   steering during the hold and verify that it queues for after-turn delivery.
   Newer guidance from the same sender replaces the held steer. With an
   exhausted cap and no persisted pause, verify a human head can run while
   the first budget-covered batch follower and later entries remain queued.
5. Restart the real daemon and repeat admission, then try operator resume on the
   same local day. The pause and exhausted cap remain, and resume is rejected.
6. Advance the injected clock past local midnight. Confirm it stays paused until
   an operator resumes, then verify the retained steer runs after the active
   turn without another restart. Confirm profile B and an independently bound
   instance were never charged or held.
7. With unknown provider price, verify a positive token limit still trips while
   the USD subtotal marks unpriced usage. Verify optional caps remain absent
   and behavior unchanged when disabled.
8. Manually inspect HTTP/UDS, CLI `show -o json`, compact native-tool instance
   status and the profile-scoped bell. Complete the product visual QA before
   changing `qa_status` to `pass`.

The automated journey boots the actual daemon and ACP test provider. It uses
HTTP to configure the limit, read the pause and resume. On systems with Unix
socket support, it also checks the status payload over UDS. Run the remaining
CLI, native-tool and visual checks before updating the scenario verdict.
