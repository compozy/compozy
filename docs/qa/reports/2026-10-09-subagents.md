# QA Report — Subagents real-provider walk (2026-10-09)

- **Scope:** branch run for the Subagents feature (`sa-qa` @ c17af94ec, snapshot of integrated `subagents`).
- **Build:** `make build` + `make web-build`; daemon served `web/dist` via `COMPOZY_WEB_DIST_DIR`.
- **Lab:** `eng-qa-bootstrap` targeted profile (cli, api, web, runtime, provider), isolated home/port/socket
  (`compozy-subagents-qa-20261009-091012-908899`, HTTP 64584). Torn down clean (`teardown.json clean: true`), then `make qa-reap`.
- **Providers:** real Claude (`claude-agent-acp`, Sonnet 5.5, operator native sign-in) as parent; real Codex
  (`gpt-5.6-sol`, medium) and Claude as children.
- **Browser:** headless Chromium (Playwright) 1440×900 at 2× DPR, light theme.

## QA-local patched build (not committed)

The stock binary blocks every non-native-tool surface (BUG-20261009-subagent-routes-unavailable), and real
Claude delegations never project a card (BUG-20261009-subagent-card-hosted-tool-name). To walk the rest, a
throwaway worktree added two QA-only changes: `state.deps.Subagents = state.subagents` in
`prepareServerDependencies`, and hosted-MCP prefix stripping in `ui_messages_subagent.go`. Everything marked
"(patched)" below ran on that build. The patch is not part of this branch.

## Matrix

| Scenario | Persona | Verdict | Bugs |
| --- | --- | --- | --- |
| RT-subagent-delegate | Ada | fail | routes-unavailable; idempotency-default; status-empty-input; show-needs-workspace; runtime-speed-empty |
| RT-subagent-restart | Dora | fail | daemon-stop-cancels; crash-no-wake |
| ET-web-subagent-card | Bruno | fail | routes-unavailable; card-hosted-tool-name; card-live-missing; running-count-settled; preview-raw-markdown |
| ET-web-native-subagent | Rafa | fail | native-subagent-title-task; preview-raw-markdown |
| ET-web-session-sidebar-threads (subagent parts) | Bruno | fail | routes-unavailable |

## Session debriefs

**RT-subagent-delegate.** Golden path on the stock binary: Claude read the brief, called
`compozy__subagent_capabilities`, delegated to Codex async (`running`, `sub-e94cc5c5e257fd21`), listed its own
edge cases, and ended its turn. Codex settled in 1m 28s; exactly one `subagent_wake` arrived with
`Subagent "Codex: diff panel default review" (sub-…) finished: completed.` / `Call compozy__subagent_status to read its result.`;
status returned `completed`, `result_available`, `acknowledged`; the parent summarized. Child context: the
first prompt is `Act as the review subagent for this task.` + the task only; `spawn_role: subagent`. Delegate
latency: 2.35 s from final tool input to result (3.7 s from call start, including streamed arguments). The
first delegate failed on the missing idempotency default; the CLI/HTTP/UDS routes all returned 503.
(patched) CLI list/`--json` (`next_cursor: null`)/`show`/`cancel --reason` (`Cancel requested for …`, then
`canceled`/`disposed`), unknown subagent → exit 1 `subagent_not_found: subagent sub-nope not found`, unknown
session → exit 1 `session_not_found`, `--limit 201` rejected; HTTP `subagents=exclude` omits subagent sessions
and carries `subagent_summary`, `subagents=bogus` → 400, unknown cancel → 404, terminal cancel → 202 with the
terminal status. Three delegations in one turn produced three separate wakes (settles were 20–40 s apart;
the last settled mid wake-turn and was queued, then delivered once). Wait mode: an inline completion after
92 s, and a 124.3 s blocking hold that returned `running` + `wait_timed_out: true` with the child still running
and a single wake at settle — no provider abort on Claude.

**RT-subagent-restart.** (patched) Reads survived restarts. A clean `daemon stop` with a child running
settled it `canceled`/`disposed` ("session canceled by user"). `kill -9` left every session `stopped/failed`;
the row became `failed` ("daemon crashed while session active"), `disposed`, logged
`subagent.recovered{reason=child_reconciled}`, and no wake.

**ET-web-subagent-card.** Stock build: no card, banner, chip, or roster. (patched) Running card with ticking
elapsed, Completed card with frozen elapsed (inside the turn fold, as `_uiux.md` S1 allows for settled
cards), group `3 subagents · 2 working · 1 done` with stacked marks and expansion, hover card (model ·
effort, status, elapsed), drill-in in the same window with `Subagent of <parent> · Open parent`, the sidebar
revealing only the opened child, and the waiting banner `Waiting on 2 subagents` whose Stop showed
`Stopping…` and canceled both. Card buttons expose `aria-label="Open <title>"` and the status as
`aria-description`. Live cards only appear after the delegating turn ends; the running count includes settled
children; previews show raw markdown.

**ET-web-native-subagent.** (patched) A real Claude Agent call produced a `provider_native` record
(`child_session_id: null`, `provider_tool_call_id: toolu_…`), a Claude-marked card expandable inline with its
own Read row, hover `Not reported` model, no wake. Title stays `Task`.

**ET-web-session-sidebar-threads.** (patched) Subagent sessions stay out of the list; the parent chip shows
`1/1`, `3/3`, `2/3`, the failed glyph with `3` and aria `3 subagents, 1 failed`; hover lists every subagent;
the chip hides once all settle cleanly; clicking opens the inspector Subagents section with
`Previous subagents (3)`. Stock build: no chip (no summary).

## Not verified (time-box)

Hooks (`spawn.pre_create`, `subagent.settled`), `result_max_chars` config, permission narrowing/escalation,
depth 2, scale (10/20), waiting-for-approval child, steer routing, parent stop cascade and interrupt
disposal, stale-reserved/orphan recovery, ⌘-click new window, keyboard focus/Escape on hover, stop-failure
toasts, search nesting, archive rules, Codex-parent tool-name shape, and board VC pixel comparison. Most
have automated owners in `_tests.md`.

## Final status

Fail — 12 bugs filed: High 4 (routes-unavailable, card-hosted-tool-name, idempotency-default,
daemon-stop-cancels), Medium 3, Low/Cosmetic 5. The two P0s block the feature for every real Claude user.
