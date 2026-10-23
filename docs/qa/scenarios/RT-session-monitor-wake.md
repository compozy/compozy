---
id: RT-session-monitor-wake
area: RT
title: A daemon-owned monitor wakes an idle agent session when a background command finishes
persona: Ada
journey: J-13
expected: An agent that runs a long command through `compozy__terminal_exec` (`still_running`) and registers `compozy__monitor_create` on the returned terminal can end its turn; when the process exits (or its output matches, or a timer elapses) the daemon starts a new turn in the same session with a `monitor` synthetic wake (`[COMPOZY MONITOR - NOT USER INPUT]`, monitor id, status, untrusted output tail), the Web transcript shows a "Monitor fired · <description> · <outcome>" divider before the agent's report, a busy session runs the wake after its current turn, a full input queue retries the wake (`delivering`), stopping the session cancels its monitors, and Claude sessions never expose the provider-internal Monitor/ScheduleWakeup/Cron tools, refuse Bash and PowerShell `run_in_background` with guidance toward `compozy__monitor_create`, and keep background Agent subagents. While an idle session has an active or delivering monitor its canonical badge is `waiting`, `waiting_on` is `monitor`, and its list row shows the amber `monitoring` mark instead of `done`, and the composer shows a monitor button whose popover lists each monitor and cancels it without waking the session.
entry_points: native tools compozy__monitor_create / compozy__monitor_list / compozy__monitor_cancel (toolset compozy__sessions); compozy__terminal_exec; Web session transcript, session list row, composer monitor popover; HTTP/UDS `GET|POST /api/workspaces/{ws}/sessions/{id}/monitors[/{monitor_id}/cancel]`; claude-agent-acp session/new + session/load `_meta.claudeCode.options`
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: 2026-10-20 isolated lab (branch feat/session-monitor, real claude-agent-acp + codex-acp) — step 1 pass (Claude: terminal_exec still_running → monitor_create → turn ended; wake 45s later with `exited with code 0` and `build-finished-7731`, agent reported unprompted), step 3 pass (Codex: wake `mon-d31de57c72c5`, reply `codex-done-5521`), step 4 timer leg pass (15s timer → `timer fired 4410`), step 7 pass (Claude reports no Monitor/ScheduleWakeup/CronCreate and no Bash run_in_background). Remaining live: step 2 Web row (covered by session-thread.test.tsx), step 4 match leg, steps 5–6 (covered by internal/monitor and daemon waker unit suites). 2026-10-21 isolated lab (branch feat/session-monitor-visibility, real claude-agent-acp sonnet) — step 8 pass: agent registered `mon-57506a4b5f2f` (900s timer); catalog row `badge=done` + `monitor_summary.active=1` rendered `data-badge="monitoring"` (amber radar), composer showed "Monitors, 1 active", popover read "QA: check CI on PR #875 · Timer · fires in 11m"; × cancel returned the monitor `canceled` over HTTP, removed the composer button, and the row left `monitoring` with no wake turn.
last_report:
overlaps: RT-020; ET-agent-terminal-window-materialization
---

story: As an operator I ask an agent to run something slow and report back, and the report arrives without me nudging the session.

1. **Terminal exit wake (Claude).** In an isolated lab, start a Claude session and prompt: "Run `sleep 20 && echo done-xyz` and tell me the output when it finishes." Expect the agent to call `compozy__terminal_exec` (result `still_running` with `terminal_id`), then `compozy__monitor_create` with `kind: "terminal"`, and end its turn. About 20s later, without any operator input, a new turn starts; `compozy session events <id>` shows a synthetic prompt whose metadata carries `kind: "monitor"`, `reason: "monitor_exit"`, and the monitor id; the agent's reply mentions `done-xyz`.
2. **Web row.** Open the session in Web: the wake renders as one divider "Monitor fired · <description> · Process exited" (the `[COMPOZY MONITOR…]` text is never shown) followed by the agent's turn.
3. **Codex parity.** Repeat step 1 on a Codex session; the same wake arrives.
4. **Timer and match.** `compozy__monitor_create {kind:"timer", delay_seconds: 10, description: …}` wakes after ~10s with "Timer elapsed"; a `until:"match"` monitor on a terminal that prints the pattern wakes with "Output matched".
5. **Busy and stopped sessions.** Fire a monitor while the session runs a turn: the wake queues and runs after it. Stop the session before the monitor fires: no turn starts, and `compozy__monitor_list` from a resumed session shows no monitor from before the stop.
6. **Cancel and limits.** `compozy__monitor_cancel` on an active monitor returns `canceled` and no wake ever arrives; a ninth active monitor in one session, or a fifth on one terminal, returns `tool_conflict`.
7. **Claude provider tools.** In a Claude session, Monitor, ScheduleWakeup, and CronCreate/CronDelete/CronList are absent; a Bash or PowerShell call with `run_in_background: true` is denied with guidance to use `compozy__terminal_exec` + `compozy__monitor_create`; a background Agent subagent still runs and returns inside its turn.
8. **Operator visibility and cancel.** Register a 10-minute timer monitor and let the turn end. The session row's mark reads `monitoring` (amber radar), not `done`; the composer shows "Monitors, 1 active". Open the popover: the row shows the description and "Timer · fires in 10m". Click × : the row disappears, the button goes away, the list row returns to `done`, and no wake turn ever starts. `GET …/monitors` then reports the monitor as `canceled`.

Sessions Kanban regression (pending integrated re-walk): open a monitor immediately before the turn
settles, without waiting for a supervision sweep. Read the session over CLI and HTTP: expect
`badge=waiting`, `waiting_on=monitor`, and an `active_monitor` work signal. No finished delivery may
appear. `session wait --until idle` must return. Cancel the last monitor without prompting: the next
supervision pass publishes exactly one `waiting → done` edge for unseen work. Repeat after marking
the settled turn seen: closure yields `idle`, without a finished delivery. A failed signal inspection
must yield `needs-attention`, never `done`.
