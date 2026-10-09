# QA Report — Subagents re-walk, round 2 (2026-10-09)

- **Scope:** retest of the 12 bugs from `2026-10-09-subagents.md` plus journeys not walked in round 1.
- **Build:** stock `sa-qa` @ 980d51fbe (`make build`, `make web-build`). No QA-local patches.
- **Lab:** `eng-qa-bootstrap` targeted profile (`compozy-subagents-qa-r2-20261009-112232-721051`, HTTP 64071), torn
  down clean (`teardown.json clean: true`), then `make qa-reap` (all clean).
- **Providers:** real Claude parent (claude-agent-acp; agent model pinned `claude-sonnet-5-5`, then `sonnet` after the
  adapter switched to aliases mid-run) and real Codex (`gpt-5.6-sol`) and Claude children.
- **Browser:** headless Chromium 1440×900 @2×, light theme. All 12 PR screenshots recaptured on this build.

## Matrix

| Scenario | Verdict | Open bugs |
| --- | --- | --- |
| RT-subagent-delegate | fail | subagent-capabilities-oversized; subagent-settled-hook-canceled; subagent-deny-message-prefixed |
| RT-subagent-restart | pass | — |
| ET-web-subagent-card | fail | subagent-card-live-missing (partly fixed) |
| ET-web-native-subagent | pass | — |
| ET-web-session-sidebar-threads (subagent parts) | pass | — |

## Retest of round-1 bugs

| Bug | Result |
| --- | --- |
| subagent-routes-unavailable | verified |
| subagent-card-hosted-tool-name | verified |
| subagent-idempotency-default | verified |
| subagent-card-live-missing | **re-found, partly fixed** (composer-submitted turns) |
| subagent-running-count-settled | verified |
| subagent-daemon-stop-cancels | verified |
| subagent-crash-no-wake | verified against the controller decision |
| native-subagent-title-task | verified |
| subagent-preview-raw-markdown | verified |
| subagent-status-empty-input | verified |
| subagent-show-needs-workspace | verified |
| subagent-runtime-speed-empty | verified |

## Newly walked

Steer routing (wake steered into a running turn: `transcript_marker.prompt_steered`, no new input row, delivery
`delivered`); parent stop cascade (3 live children `canceled`/`disposed`, child sessions stopped, no wake); interrupt
disposal (child kept running and completed, delivery `disposed`, no wake, result readable); ⌘-click (child opens in a
second tiled window); keyboard focus opens the hover card and Escape closes it; a `spawn.pre_create` deny →
`capability_denied`; `result_max_chars = 1000` → 1000-character result, `result_truncated: true`, history hint; `999`
rejected with the documented message; depth 2 (grandchild `depth: 2`); `invalid_request` on an idempotency-key
mismatch; `permission_escalation_denied` for an atom outside the budget; `agent_not_found`, `model_unavailable`,
`subagent_not_found`.

## New findings

- **BUG-20261009-subagent-settled-hook-canceled (High):** `subagent.settled` async hooks are canceled before they run
  (`subagent_publish.go:101-104`, deferred cancel of `hookCtx`).
- **BUG-20261009-subagent-capabilities-oversized (Medium):** the capabilities answer is 99,133 characters
  (`opencode` alone lists 611 models), over Claude's MCP result limit.
- **BUG-20261009-subagent-deny-message-prefixed (Low):** the deny message carries `hooks: event "spawn.pre_create" denied: `.
- Observation: interrupting the parent turn while a delegate call is still starting its child fails that row
  (`context canceled` in ACP initialize). Interrupting after the child is running behaves as specified.

## Latency

Final tool input → delegate result: 1.8–2.5 s for the first delegate in a turn, 3.3–4.1 s for a back-to-back second
(spawn lock), 4.6 s for the first-ever Codex launch in the lab, and ~10 s for a Claude child. Round 1 measured the
2-minute `mode=wait` hold on Claude (124.3 s, no abort).

## Not walked

Restart steps 4–5 (acpmock seeding; IT-031), native cancel 409 and fixture fallback, the 10/20-delegation scale run,
a waiting-for-approval child, stop-failure toasts, search nesting, archive rules, and the 10-minute wait.

## Final status

Fail. Two scenarios are open: RT-subagent-delegate (1 High, 1 Medium, 1 Low new bugs) and ET-web-subagent-card
(1 Medium re-found). 11 of the 12 round-1 bugs are verified.
