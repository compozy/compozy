---
id: RT-subagent-delegate
area: RT
title: Delegate to a subagent, get woken, and read the result
persona: Ada
journey: J-15-operate-session-via-cli-api
expected: From an active turn, compozy__subagent_capabilities reports the caller's inherited runtime, depth, live count, and per-agent/provider can_delegate with the exact constraint strings; compozy__subagent_delegate in async mode returns status running with subagent_id and child_session_id within 3 s on a warm provider, the child's first prompt is exactly the task (role prefix when role is not general, no parent history, no attachments), the parent turn completes independently, and when the child settles the parent receives exactly one pointer wake per batch (steered into a running turn only when the agent declares steer_ext or concurrent_prompt and no user steer is queued, otherwise queued ahead of user prompts) whose text matches the fixed wake lines; compozy__subagent_status returns the result capped at [subagents].result_max_chars and moves delivery to acknowledged; wait mode returns the result or wait_timed_out without canceling the child; cancel, stop cascade, interrupt disposal, narrowing-only permissions, idempotency, and every documented error code behave as in the spec; CLI compozy session subagents list/show/cancel and the HTTP/UDS routes return the documented shapes and exit codes.
entry_points: native tools compozy__subagent_capabilities, compozy__subagent_delegate, compozy__subagent_status, compozy__subagent_cancel (toolset sessions); compozy session subagents <session-id> [--json]; compozy session subagents show|cancel <subagent-id>; compozy session list --subagents include|exclude|only; GET /api/workspaces/{workspace_id}/sessions/{session_id}/subagents and GET|POST /api/workspaces/{workspace_id}/subagents/{subagent_id}[/cancel] over HTTP and UDS; [subagents] result_max_chars; hooks spawn.pre_create (spawn_role subagent) and subagent.settled
qa_status: fail
bug_ids: BUG-20261009-subagent-routes-unavailable; BUG-20261009-subagent-idempotency-default; BUG-20261009-subagent-status-empty-input; BUG-20261009-subagent-show-needs-workspace; BUG-20261009-subagent-runtime-speed-empty; BUG-20261009-subagent-capabilities-oversized; BUG-20261009-subagent-settled-hook-canceled; BUG-20261009-subagent-deny-message-prefixed
fix_status: pending
retest_status:
fix_commits:
evidence: .compozy/tasks/subagents/orchestration/screens/pr/
last_report: docs/qa/reports/2026-10-09-subagents-r2.md
overlaps: RT-session-spawn-wake; RT-subagent-restart; ET-web-subagent-card; ET-web-native-subagent
---

Spec: `.compozy/tasks/subagents/` (`_spec.md` Business Rules 1–18 and Delivery and lifecycle rules;
`_dx.md` for exact payloads, wake text, CLI transcripts, and error messages). Walk on an isolated lab
with real Claude and Codex providers; automated owners are IT-001…IT-032 and E2E-001, E2E-007,
E2E-008, which use acpmock.

1. **Golden path (real providers).** In a Claude session, prompt: "Ask Sol in Codex for a
   recommendation and the three biggest risks on <brief>; keep working meanwhile." Expect one
   `compozy__subagent_capabilities` call, one `compozy__subagent_delegate` with `mode` async and a Codex
   target, a `running` result with `subagent_id` (`sub-…`) and `child_session_id`, and the parent's own
   work continuing. When Codex settles, exactly one wake turn arrives with
   `Subagent "<title>" (<sub-id>) finished: completed.` and
   `Call compozy__subagent_status to read its result.` (`each result` when a batch lists several
   rows); the parent calls status, gets `completed`,
   `work_state: result_available`, the result, and `delivery: acknowledged`, then answers. Record the
   delegate latency (target < 3 s on a warm provider).
2. **Child context.** `compozy session history <child_session_id> -o json`: the first user prompt is
   exactly the task (with `Act as the review subagent for this task.` when `role: review`), with no
   parent messages and no attachments. The child session has `spawn_role = "subagent"`.
3. **Batching and routing.** Delegate three subagents in one turn that settle close together: the
   parent gets one wake listing every settled row; a row that settles after that wake started forms
   the next batch. On an agent whose steer capability is `steer_ext` or `concurrent_prompt`, a
   subagent that settles while the parent is mid-turn is steered into that turn (no new input row,
   `MarkerPromptSteered` with the wake id); with a user steer already queued, or on an agent without
   steering, the wake is queued with priority ahead of user-queued prompts. A status read of a terminal
   row before its queued wake starts removes it from the wake; an emptied wake is canceled.
4. **Wait mode.** `mode: "wait"` with a short task returns the result inline and no later wake. With
   `timeout_ms: 5000` on a long task: returns `wait_timed_out: true`, the child keeps running, and a
   normal wake arrives when it settles. Measure a 2-minute and a 10-minute wait on real Claude and
   Codex; if either provider aborts the tool call, record it (Known Risk: provider MCP call timeouts).
5. **Waiting child.** A child that requests approval shows `status: waiting` and does not wake the
   parent; `compozy__session_approve` on `child_session_id` (or the operator) resolves it, and the
   child continues.
6. **Cancel, stop, interrupt.** `compozy__subagent_cancel` on a live row → `cancel_requested`, then
   `canceled`, delivery `disposed`, no wake. Stopping the parent session stops a child and grandchild
   in the same operation (no reaper tick), rows `canceled`, no wake. Interrupting a parent turn keeps
   that turn's children running; their results are readable but never wake the parent.
7. **Permissions and limits.** Omitted `tools`/`skills`/`mcp_servers`/`workspace_paths` inherit the
   caller's budget; `[]` gives none; a broader `permission_mode` or an atom outside the caller's budget
   → `permission_escalation_denied` with the exact message. No TTL, depth, or live-count cap: a
   subagent delegating its own subagent reports `depth: 2`; a `spawn.pre_create` hook patch that sets a
   TTL for the subagent role → `capability_denied`.
8. **Idempotency.** Re-sending the same delegate with the same `idempotency_key` returns the same row;
   the same key with a different task → `invalid_request`. If a `spawn.pre_create` hook denies a
   delegation while the same-key replay waits, both calls return `capability_denied` and no row remains.
9. **Errors.** Outside an active turn → `parent_not_active`
   (`Subagents require an active turn in the calling session.`); unauthenticated provider →
   `provider_unavailable`; unknown model → `model_unavailable` listing available models; unknown
   `target.agent` → `agent_not_found`; unknown id → `subagent_not_found`.
10. **CLI and HTTP/UDS (E2E-007).** With one running and one completed subagent, run the `_dx.md`
    transcripts verbatim: `compozy session subagents <session-id>` (table and `--json` with
    `next_cursor`), `--origin`, `--status`, `--limit` (max 200), `show` (human and `--json`), `cancel
    --reason` (`Cancel requested for …`), all exit 0; unknown subagent → exit 1 with
    `subagent_not_found: subagent sub-… not found`; unknown session → exit 1 `session_not_found`. The
    same reads and cancel over HTTP and UDS return identical payloads; cancel of a provider-native row →
    409 `subagent_not_cancelable`. `GET …/sessions?subagents=exclude` omits subagent sessions and
    returns `subagent_summary` on the parent; `subagents=bogus` → 400 `invalid_request`.
11. **Config (E2E-008).** `compozy config set subagents.result_max_chars 1000`, reload, delegate a
    task whose answer exceeds 1,000 characters → `result_truncated: true`, a 1,000-character result,
    and the `compozy__session_history` hint. `999` fails validation with
    `subagents.result_max_chars must be between 1000 and 1000000: 999`.
12. **Hooks.** A config hook on `subagent.settled` receives the documented payload once per settled
    subagent (delegated and provider-native); a `spawn.pre_create` hook sees `spawn_role: "subagent"`
    and the `subagent` object, and a deny blocks the delegation with `capability_denied`. A slow
    observe-only settled hook must not block parent status reads, acknowledgement, or wake delivery.
13. **Scale.** From one turn, delegate 10 and then 20 subagents concurrently; record delegate latency
    per call (spawn runs under the global spawn lock) in the QA report.

QA impact 2026-10-08 (subagents): new in this change; no prior verdict.

QA walk 2026-10-09 (real Claude parent, Codex children): golden path, child context, wake text, wait mode (92 s inline; 124.3 s timeout hold, no provider abort), CLI/HTTP reads and cancel verified on a QA-local patched build; the stock build returns 503 on every subagent route and needs an explicit idempotency_key. Verdict: fail. Report: `docs/qa/reports/2026-10-09-subagents.md`.

Fix round 1 automated scope (sa-fix-core): `TestSubagentConcurrentDenial` and
`TestSubagentObserveHook` own concurrent denial and nonblocking observer regressions.
`TestSubagentHookDaemonIntegration` exercises the real hook/ACP/store denial boundary, asserts
`capability_denied` with the hook reason alone, and waits for a real async settled subprocess
to complete successfully with its documented payload after dispatch returns.
These focused checks do not replace the real-provider scenario re-walk or change its fail verdict.

Additional fix-round automated coverage: `TestSubagentPendingInjectionDaemonIntegration` verifies
accepted pending steering and a distinct successor batch; `TestSubagentSuccessorDaemonIntegration`
verifies capability-none queueing and the fourth result after dispatch. `TestSubagentRootHostedMCPDaemonIntegration`
verifies a ROOT child's status/delegate calls with omitted keys, catalog SSE, and operator HTTP
cancellation of a live child/grandchild tree. `TestSubagentDaemonIntegration` checks IT-030's single
unseen finished root. SQLite tests reopen between failed wakes and prove abandonment on attempt 3;
service tests prove no immediate failed-wake retry and the `subagent.wake_abandoned` log.
`TestSubagentCleanRestartDaemonIntegration` resumes delegated work after clean shutdown;
`TestSubagentRecoveryDaemonIntegration` reconciles native work from a lost turn as interrupted.
These automated journeys preserve the real-provider fail verdict above until the QA owner re-walks.

Fix round 2 automated scope: cancellation returns after durable canceled/disposed acceptance;
physical child/grandchild stop completes on the daemon lifecycle even after request cancellation.
The HTTP journey waits for both terminal rows and stopped sessions. Interrupted pending steering
must return to pending, while failed wakes retry after a short backoff without another user prompt
and stop at the existing three-attempt cap. A canceled/error turn retains cancellation semantics.
Clean restart must preserve the original answer of an already completed, unobserved child.
Owning evidence: `TestSubagentCancelLifetime`, `TestSubagentPendingSteerInterruption`,
`TestSubagentWakeFailureLimit`, `TestSubagentCanceledErrorPrecedence`, and the daemon hosted-MCP,
settled-restart and clean-restart journeys. The real-provider QA verdict above is unchanged.

Re-walk 2026-10-09 (stock 980d51fbe): the five filed bugs are verified fixed. Golden path, steer routing (`prompt_steered`, delivery `delivered`), error codes, idempotency mismatch, escalation denial, `result_max_chars = 1000` truncation with hint, `999` rejected, depth 2, hook deny → `capability_denied`, cancel, and CLI/HTTP reads pass. New: the capabilities answer (99 KB) overflows Claude's MCP limit, `subagent.settled` hooks are canceled before they run, and the deny message carries an internal prefix. Verdict: fail. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.

Capabilities regression check (BUG-20261009-subagent-capabilities-oversized): inspect each provider
option from the active-turn call. Unavailable providers have `models: []`; delegable providers list
at most 40 models, with current/default first. Verify `models_total` counts the full catalog and
`models_truncated` signals omission. Delegate using a valid advertised model beyond the preview and
confirm it is accepted. Automated UT-017 covers the 611-model catalog; real-provider re-walk pending.
