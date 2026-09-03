# QA Run Report — 2026-09-01 — Terminal Rework (plain session output + agent-managed windows)

- **Scope:** ADR-019 rework — removal of agent-reported session terminal blocks (S4), daemon
  materialization of agent-opened interactive terminals as desktop windows, hosted-MCP tool-name
  and envelope normalization for the S3 supervised block, terminal tool descriptors and skill
  steering.
- **Cadence tier:** change-targeted
- **Build at walk:** working tree of `integrated-terminal` (post-ADR-019 implementation)
- **Environment:** isolated `terminal-rework` lab; API `http://127.0.0.1:57454`; daemon serving a
  fresh `web/dist`; agent = Claude Code (Haiku) over `native_cli` with operator sign-in
- **Started:** 2026-09-01T15:09:52Z
- **Status:** local targeted PASS; exact-head verification owned by PR #490 CI
- **Verdict:** PASS for all walked scenarios
- **Bootstrap manifest:** `/Users/pedronauck/dev/qa-labs/compozy-terminal-rework-20260901-150952-749450-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Teardown:** `qa-artifacts/qa/teardown.json` — `"clean": true` (one agent-nohup'd
  `http.server` survivor killed by exact PID afterwards; port re-verified free)

## Automated Evidence

| Lane | Result | Evidence |
|---|---|---|
| `make gate` (full tree, scoped lanes incl. Go ./…, JS all workspaces, codegen-check) | PASS (exit 0) | `.cache/gate/` records at walk tree |
| Go affected packages (`acp`, `transcript`, `session`, `api/*`, `acpmock`) `-race` | 3787 passed | local run 2026-09-01 |
| `internal/tools` after descriptor rewrite | 654 passed | local run 2026-09-01 |
| New `internal/daemon/terminal_window_bridge_test.go` | 17 passed (`-race`) | local run 2026-09-01 |
| Web unit/component (turbo) | 6781 passed / 745 files | local run 2026-09-01 |
| Playwright `E2E-003` with new materialization + focus assertions | PASS (1.1m, real daemon) | focused run 2026-09-01 |

## Real-User Walk (Haiku agent, lab web UI)

Prompted, in Portuguese, the exact failing ask from 2026-09-01: "do you have access to the
CompozyOS integrated terminal? open one and run a simple HTTP server for me to watch".

1. Agent answered "Sim, tenho acesso aos terminais integrados do CompozyOS", resolved
   `compozy__tool_info`, called `compozy__terminal_open`, and typed the command under a granted
   one-time typing approval — `qa/05-prompt-sent.png`, `qa/06-agent-working.png`.
2. A Terminal window for `term-51e7b88e4515` materialized on the desktop without stealing focus
   from the session, streaming live output; the agent later titled it "HTTP Server (8899)" —
   `qa/06-agent-working.png`, `qa/14-affordance-window-open.png`.
3. Closing the Terminal window left the process running (`curl :8899` → 200) and the window did
   not reopen — `qa/08-window-closed-process-alive.png`.
4. Routine internal commands (`uname -a`, `pwd`, `ls`) rendered as plain command output — no
   terminal chrome, no "reported by agent" label — `qa/09-internal-commands.png`.
5. A hidden `terminal_exec visible:false` (`sleep 25`) listed as a `mode:"pipe"` terminal and
   materialized no window; the window-manager snapshot held only the session window — API dumps in
   the walk log.
6. After the hosted-MCP normalization fix, the transcript's `terminal_open` call renders as the S3
   supervised block (MonoId + "Open terminal" jump) and terminal rows carry registered labels
   ("Typed in terminal", "Opened terminal") — `qa/11-supervised-block-live.png`,
   `qa/12-clean-labels.png`.
7. The supervised block's "Open terminal" affordance focused the existing materialized window in a
   workspace-scoped client — `qa/14-affordance-window-open.png`.

Evidence root: `/Users/pedronauck/dev/qa-labs/compozy-terminal-rework-20260901-150952-749450-lab/qa-artifacts/qa/`.

## Scenario Verdicts

| Scenario | Verdict | Notes |
|---|---|---|
| `ET-agent-terminal-window-materialization` | pass | Steps 1–6 walked live (steps above) |
| `ET-terminal-session-block-handoff` | pass | Block render, handoff (`still_running` + id), affordance focus walked; read/wait untrusted marking owned by E2E-003/IT suites (green) |
| `ET-terminal-window-native-flow` | pass | Change-driven risks walked (no double window, no resurrect, affordance dedupe); dock adoption automation (E2E-002/018/020) re-runs in PR CI |
| `SITE-terminal-docs-truth` | pass | `terminal/agents-and-safety.mdx` rewritten to the walked plain-output + window truth; site lanes green in gate |

## Fixes Landed During the Walk

- `canonicalCompozyToolName` — hosted-MCP tool names (`mcp__<server>__compozy__*`) normalize to
  their native ids for S3 interception and row labels.
- `readTerminalEnvelope` — decodes JSON-string MCP envelopes (`raw_output` as string) so
  `terminal_id`/`still_running` reach the supervised block.
- Registered row labels for the remaining `compozy__terminal_*` tools.

## Known Limitations

- In a Global-scope (workspace-unbound) client, the transcript's "Open terminal" affordance does
  not open a window; scoping to the workspace restores it. Consistent with the window model's
  workspace binding; not a regression.
