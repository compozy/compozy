# BUG-20261009-subagent-routes-unavailable: Every subagent read/cancel route answers 503 Subagents are unavailable

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Ada
- **Journey Step:** see scenario
- **Scenarios:** RT-subagent-delegate; ET-web-subagent-card; ET-web-session-sidebar-threads
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

Ada asks `compozy session subagents <parent>` (and the Web asks the same HTTP routes) and gets `feature_unavailable: Subagents are unavailable.` even though the agent's native tools delegate and wake correctly. Because the Web roster, SSE snapshot, and `subagent_summary` share the same nil service, the parent shows no card state, no chip, no waiting banner, and no inspector roster.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. Start a daemon from the branch binary; in a Claude session delegate one subagent.
2. `compozy session subagents <parent>` / `GET /api/workspaces/{ws}/sessions/{id}/subagents` (HTTP and UDS).

**Expected:** 200 `SubagentListPayload`; CLI table.
**Actual:** 503 `{"code":"feature_unavailable","error":"Subagents are unavailable."}` on HTTP and UDS; CLI exit 1.

## Evidence

- Root cause: `bootRuntimeFoundation` builds `state.deps` (`internal/daemon/runtime_dependencies.go:34` copies `state.subagents`, still nil) before `bootSubagents` (`internal/daemon/boot_components.go:20`) assigns it; `bootServers` then passes the stale nil via `server_options.go:15,81`. Native tools are unaffected because `native_tools_dependencies_builder.go:17` uses a closure. A one-line QA-local patch (`state.deps.Subagents = state.subagents` in `prepareServerDependencies`) made every route answer 200; the rest of the walk ran on that patched build.
- Report: `docs/qa/reports/2026-10-09-subagents.md`
