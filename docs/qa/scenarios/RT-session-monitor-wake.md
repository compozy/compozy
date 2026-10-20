---
id: RT-session-monitor-wake
area: RT
title: A daemon-owned monitor wakes an idle agent session when a background command finishes
persona: Ada
journey: J-13
expected: An agent that runs a long command through `compozy__terminal_exec` (`still_running`) and registers `compozy__monitor_create` on the returned terminal can end its turn; when the process exits (or its output matches, or a timer elapses) the daemon starts a new turn in the same session with a `monitor` synthetic wake (`[COMPOZY MONITOR - NOT USER INPUT]`, monitor id, status, untrusted output tail), the Web transcript shows a "Monitor fired · <description> · <outcome>" divider before the agent's report, a busy session runs the wake after its current turn, a stopped session is never revived (the monitor reads `undelivered`), and Claude sessions never expose the provider-internal Monitor/ScheduleWakeup/Cron tools or Bash `run_in_background`.
entry_points: native tools compozy__monitor_create / compozy__monitor_list / compozy__monitor_cancel (toolset compozy__sessions); compozy__terminal_exec; Web session transcript; claude-agent-acp session/new + session/load `_meta.claudeCode.options`
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: 2026-10-20 isolated lab (branch feat/session-monitor, real claude-agent-acp + codex-acp) — step 1 pass (Claude: terminal_exec still_running → monitor_create → turn ended; wake 45s later with `exited with code 0` and `build-finished-7731`, agent reported unprompted), step 3 pass (Codex: wake `mon-d31de57c72c5`, reply `codex-done-5521`), step 4 timer leg pass (15s timer → `timer fired 4410`), step 7 pass (Claude reports no Monitor/ScheduleWakeup/CronCreate and no Bash run_in_background). Remaining live: step 2 Web row (covered by session-thread.test.tsx), step 4 match leg, steps 5–6 (covered by internal/monitor and daemon waker unit suites).
last_report:
overlaps: RT-020; ET-agent-terminal-window-materialization
---

story: As an operator I ask an agent to run something slow and report back, and the report arrives without me nudging the session.

1. **Terminal exit wake (Claude).** In an isolated lab, start a Claude session and prompt: "Run `sleep 20 && echo done-xyz` and tell me the output when it finishes." Expect the agent to call `compozy__terminal_exec` (result `still_running` with `terminal_id`), then `compozy__monitor_create` with `kind: "terminal"`, and end its turn. About 20s later, without any operator input, a new turn starts; `compozy session events <id>` shows a synthetic prompt whose metadata carries `kind: "monitor"`, `reason: "monitor_exit"`, and the monitor id; the agent's reply mentions `done-xyz`.
2. **Web row.** Open the session in Web: the wake renders as one divider "Monitor fired · <description> · Process exited" (the `[COMPOZY MONITOR…]` text is never shown) followed by the agent's turn.
3. **Codex parity.** Repeat step 1 on a Codex session; the same wake arrives.
4. **Timer and match.** `compozy__monitor_create {kind:"timer", delay_seconds: 10, description: …}` wakes after ~10s with "Timer elapsed"; a `until:"match"` monitor on a terminal that prints the pattern wakes with "Output matched".
5. **Busy and stopped sessions.** Fire a monitor while the session runs a turn: the wake queues and runs after it. Stop the session before the monitor fires: no turn starts, and `compozy__monitor_list` from a resumed session shows the monitor `undelivered`.
6. **Cancel and limits.** `compozy__monitor_cancel` on an active monitor returns `canceled` and no wake ever arrives; a ninth active monitor in one session returns `tool_conflict`.
7. **Claude provider tools.** In a Claude session, ask the agent to list its tools: Monitor, ScheduleWakeup, CronCreate/CronDelete/CronList are absent and Bash has no `run_in_background` parameter.
