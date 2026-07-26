# BUG-20260713-cursor-agent-mode-unavailable: Cursor sessions cannot leave Ask mode to perform work

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-17 create and use a session, step 3
- **Scenarios:** RT-cursor-agent-mode
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** n/a

## Summary

Bruno successfully started Cursor with Grok 4.5 and asked it to inspect existing evidence then write a release-risk brief. Cursor completed the analysis but refused the write because `Ask mode is on`, instructing Bruno to switch to Agent mode. The same session was then made the exact owner of a real child task; it read the task through `agh__task_read` but refused the mutating claim/complete operations for the same reason. The AGH session UI exposes provider, model, and reasoning selection but no Cursor mode control, so the visible instruction cannot be followed and both normal implementation tasks and durable Task execution are blocked.

## Reproduction

- **Charter:** CH-cursor-agent-mode · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US; live Cursor ACP with `grok-4.5[effort=high,fast=true]`.

1. Start a Cursor/Grok 4.5 session from Agents.
2. Ask it to inspect existing files and create one new report without changing existing files.
3. Observe the provider response and inspect the session/runtime controls.
4. Assign a queued Task to the same exact session and ask it to claim/complete via AGH tools.

**Expected:** A session intended for agent work starts in a writable mode or exposes the provider's supported mode selector before/after creation.
**Actual:** Cursor reports that Ask mode prevents the requested write and tells the user to switch modes, but AGH offers no such control.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-new-session-grok-transcript.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/journey-log.jsonl`
- Session `sess-b1c980b86709053d` returned a complete draft but created no `reports/launch-risk-brief.md`.
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-claim-blocked-ask-mode.dom.txt`
- Task `task-f6638f9897b1b0f8` remained Ready with run `run-0dc2db2a608bf620` unclaimed after the session read it successfully.

## Fix

- **Root cause:** Confirmed in ACP configuration negotiation. The always-installed tool gateway prioritized `default`/`ask` ahead of Cursor's advertised `agent` mode, so approve-all sessions were explicitly downgraded to Ask even though the provider contract supports Agent. The post-negotiation capability snapshot also remained stale after mode/model mutations.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** Canonical ACP negotiation suites assert permission-first selection of Cursor `agent`, provider-native defaults, reconciled current mode/model snapshots, and no redundant configuration RPCs. Model-catalog/session-manager suites also reject unsupported authoritative Cursor models before process startup. An isolated live Cursor session completed a real `Edit File` call and reported current mode `agent`; controller UI replay remains the final persona-level assertion.

## Verification

- An isolated live Cursor/Grok session negotiated current mode `agent` and completed a real `Edit File` operation.
- The final same-persona Browser replay created multiple task-role sessions that claimed their exact existing runs through AGH's mutating native tools and completed exactly once; no session reported Ask-mode refusal.
