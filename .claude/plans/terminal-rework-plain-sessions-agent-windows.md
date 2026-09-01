# Terminal rework: plain session output + agent-managed terminal windows

Branch: `integrated-terminal` (PR #490, open). Spec dir: `.compozy/tasks/integrated-terminal/`.

## Context

Real use of the integrated-terminal build surfaced two product failures (Pedro, 2026-09-01):

1. **Session transcripts drown in terminal chrome.** The `_meta.terminal_output` opt-in (`internal/acp/start_process.go:200-203`) makes the flagship ACP adapter tag every internal Bash command, producing "Used Terminal" rows plus an xterm "reported by agent" block per command (S4, task_07). The spec designed this deliberately (US-025, ADR-012 door 3) and marked it **independently pullable** — after seeing it live, the verdict is: revert to the pre-branch plain output rendering.
2. **The headline user ask — agent opens and manages real terminal windows — doesn't work.** All building blocks exist (11 `compozy__terminal_*` tools exposed by default via hosted MCP; daemon-authoritative `internal/windowmanager` with `compozy__window_*` tools), but: agent-created terminals never materialize a window (catalog entry only — `_uiux.md` S1 even promised "window appears immediately" for `exec visible:true`, so this is an implementation gap); tool descriptors never say a window/desktop is involved; the spec actively forbids "unilaterally opening windows" (US-016.EC-1); and no playbook stitches terminal_open → visible window. Result: an agent asked "do you have access to the integrated terminal?" just runs internal commands.

Reference patterns (explored: `.resources/orca`, `.resources/herdr`, `.resources/cmux`) converge: **chat renders plain collapsible text; real execution lives in real, visible, unfocused panes/windows; creation never steals focus; a skill teaches the playbook; reads are bounded; creation responses report visibility truth.**

## Part A — Session transcript: plain output again (pull S4)

Goal: agent-internal commands render exactly as pre-branch (`bash-content.tsx` → `DetailPre`). Keep S3 (deliberate `compozy__terminal_exec/open` supervised block with live attach + "Open terminal" jump) — that is the good, rare path. Keep ACP door 1 (`terminal/*` pipe methods, pinned by IT-011) untouched.

Delete targets (Go):

- `internal/acp/start_process.go:200-203` — drop the `Meta: {"terminal_output": true}` opt-in (keep `Terminal: true`).
- `internal/acp/agent_reported_terminal.go` — whole file (ingestion, 64KB tail cap, suppression predicate).
- `internal/acp/handlers_session_state.go:53-66` + `handlers_session_update.go` — remove agent_reported branch/suppression wiring.
- `internal/api/core/prompt_stream_emit.go:131-142` — SSE `agent_reported` part emission.
- `internal/transcript/ui_messages.go:211-228` — replay of reported blocks.
- `internal/testutil/acpmock/` — terminal_output meta support.

Delete targets (web):

- `web/src/systems/session/components/session-agent-reported-block.tsx` (+ stories).
- `agent_reported` origin branches: `lib/session-data-renderers.tsx:22-24`, `components/assistant-ui/session-timeline-render.tsx:66-72`, timeline fold/work/summary helpers.
- `AgentEventPayload.reported_terminal` + `origin: "agent_reported"` from session types/schemas.

Tests/QA:

- Remove S4-bound: UT-084, IT-024, E2E-010, VC-01, VC-02; `session-data-renderers.test.tsx` reported cases; `docs/qa/scenarios/ET-agent-reported-terminal.md` (delete).
- Edit `docs/qa/scenarios/ET-terminal-hook-events.md` step 6 (referenced the reported block).
- Keep/verify plain-path rendering tests (`tool-call-card.test.tsx`, bash-content).

## Part B — Agent-opened terminals materialize as desktop windows

Mechanism (daemon-side, matches "daemon truth wins" + spec's promised behavior): when an **agent-actor interactive PTY terminal** is created (`terminal_open`, `terminal_exec visible:true`), a daemon glue subscriber opens a managed window in that workspace's `internal/windowmanager` model: `app:"terminal"`, `instanceKey:<terminal_id>`, `route:/terminal/<id>`, **no focus command** (opening mutates topology only; focus is client-scoped and untouched). One-shot at creation — the only trigger is the opened event, so a human-closed window never resurrects. Window-open failure logs (Warn) and never fails the tool call. Pipe terminals (yielded execs, ACP door 1) and human/CLI actors do not materialize (v1 scope).

Wiring (design verified against source by Plan agent):

- New `internal/daemon/terminal_window_bridge.go` (~120 lines) + `terminal_window_bridge_test.go`; register `attachTerminalWindowBridge(state.terminals, state.windowManagers, state.logger)` in `bootHooks` beside the existing `attachTerminalHookBridge` (`internal/daemon/boot_hooks.go:47`; observer set freezes at first Notify — boot order verified safe).
- Filter: `event.Kind == EventKindOpened && event.Actor.Kind == ActorKindAgent && event.Detail.Mode == ModePTY` (fires from `manager_open.go:129` and `manager_exec.go:290` visible-only; ACP `manager_pipe.go:96` is always pipe; pipe→pty promotion doesn't exist).
- Command: reuse `windowManagerProvider` (`native_tool_window_manager_execute.go:13`) → per-profile `windowmanager.Manager`; mirror the `ReconcileDeletedSession` snapshot→Execute CAS loop (`session_delete_reconciliation.go:40-61`, `ExpectedRevision`, ~3 retries on `ErrRevisionConflict`); `Origin:"terminal.open"`; `Route.Search` must be non-nil map; zero `FloatingRect` gets sane clamp defaults.
- Dedupe inside the CAS loop: skip if any `snapshot.Windows` entry has `App=="terminal"` + same `InstanceKey` (minimized counts — respects human intent).
- Desktop: if `BoundRun.SessionID` matches a `session`-app window's InstanceKey, use that window's desktop (terminal lands beside the session the human is watching); else empty → first standard desktop.
- Sync observer, `context.WithoutCancel` + ~5s timeout (mirrors `window_manager_hooks.go:20`); `errWindowManagerProfileDeleted`/`ErrWorkspaceNotFound` are benign skips.

Web: no new mechanism — window-model SSE already projects new windows to every client of the workspace (and the durable model shows the window on next load if no client was connected); unknown terminal id renders `TerminalNotFoundState` and self-heals when the catalog lands (`use-terminal-window-app-state.ts:104-108`), so SSE ordering is a non-issue. The transcript S3 block "Open terminal" affordance remains the in-session jump.

E2E/IT:

- New canonical suite `internal/daemon/terminal_window_bridge_test.go` (real `windowmanager.Manager` over `memory_repository.go`, events through a real `terminal.Notifier`): window created with app/instanceKey/route/desktop; duplicate event → no second window; pipe/human/system events → no window; provider failure → no panic; focus untouched.
- Go IT: session-bound agent (acpmock) calls hosted `compozy__terminal_open` mid-turn → terminal record + window in snapshot (proves `ErrHostedRunRequired` run-binding works live).
- Web/daemon E2E: agent exec visible → window appears on desktop without focus steal; human closes window → terminal lives, no re-open.

## Part C — Steering: make agents actually reach for it

1. **Tool descriptors** (`internal/tools/builtin/terminal.go`): rewrite descriptions to lead with the capability — e.g. terminal_open: "Open a persistent interactive terminal that appears as a Terminal window on the user's CompozyOS desktop; the user watches live and can take over…" — keep safety clauses. Refresh descriptor digests/pinned tests + `make codegen` if generated.
2. **Official skill** `skills/compozy/references/terminal.md`: add the windows/desktop truth + a management playbook (open→window materializes; write/read/wait loop; request_input; list-before-open; never close terminals you didn't open; focus discipline), and soften the activation rule per the new posture.
3. **Spec corrections** (same change, hard cut): amend `_spec.md` Core Feature 7 + Business Rule 9 wording (journal/list invariants stay; "terminal-styled blocks" → plain output), `_uiux.md` S4 removed, US-025 withdrawn, US-016.EC-1/AC-1 reworded (user-asked terminal work opens windows directly; offer-first only for ambiguous watching), ADR-001 mitigation line, ADR-012 door 3 marked replaced. Add `adrs/adr-019.md` recording both reversals with the 2026-09-01 evidence. Site docs: `packages/site/content/docs/terminal/*.mdx` claims updated (SITE-terminal-docs-truth reset).

## QA tracker impact

Changed behavior → reset: `ET-terminal-session-block-handoff.md` (S3 unchanged but neighbors changed — verify), `ET-terminal-window-native-flow.md`, `SITE-terminal-docs-truth.md`. Delete: `ET-agent-reported-terminal.md`. New untested scenarios: agent-terminal-window-materialization; session plain command output. Walk all before close.

## Compozy Impact Audit

- Native tools: `compozy__terminal_*` description text (11 descriptors) — schemas/IDs unchanged; digest-pinned tests refreshed. `compozy__window_*` untouched.
- Extensibility/hooks: no new hook events; terminal `EventKindOpened` gains a daemon subscriber; window commands reuse existing model; official skill updated.
- Workspace data isolation: window opens keyed by the terminal's workspace/profile scope; no new data class; catalog/window SSE scoping unchanged.
- Config lifecycle: **zero new config keys** (materialization is spec-promised default; references ship it ungated). `[terminal]` keys untouched.
- Web/Docs impact: session rendering revert (web), window materialization (web receives via existing SSE), site terminal docs updated.

## Verification

- `make gate` locally; PR #490 CI green at final head.
- Walk reset/new QA scenarios per `qa-execution` with a real daemon + web (agent session: "open a terminal and run X, watch it") — the exact failing flow from the screenshot must now pass.

## Execution order

1. Part A revert (self-contained; commit 1).
2. Part B daemon glue + web verification + tests (commit 2).
3. Part C descriptors + skill + spec/docs + QA scenario updates (commit 3).
4. QA walk + gate + push; PR #490 stays the delivery vehicle.
