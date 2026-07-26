---
name: 12-web-ui
title: Web UI (React 19 SPA) — Real-LLM Pre-release QA Plan
description: Behavior-first QA scenarios for the AGH web app — TanStack Router file-based routes, app-renderer-systems (per-domain queries/mutations/optimistic), TanStack Query v5 server state, Zustand UI state, openapi-fetch typed contract, assistant-ui SSE chat runtime, shadcn/@agh/ui primitives, Tailwind v4 tokens — driven against a real Claude Code ACP subprocess through the same daemon HTTP/SSE the operator hits in production. Closes the loop end-to-end so every truthful-UI invariant, accessibility contract, COPY.md vocabulary rule, DESIGN.md token rule, and SSE/after_seq replay invariant is proven by browser-side scenarios, not by isolated component tests.
type: final-qa-child
module: web-ui
parent: ../_parent.md
provider_lanes: [claude-code]
authoritative_runtime_truth:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/web/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/web/AGENTS.md
  - /Users/pedronauck/Dev/compozy/agh/DESIGN.md
  - /Users/pedronauck/Dev/compozy/agh/COPY.md
references:
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/03-acp-sessions.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/11-api-cli-parity.md
---

# 12 — Web UI (React 19 SPA)

Sibling of `03-acp-sessions.md` (real ACP behavior end-to-end through the prompt path) and `11-api-cli-parity.md` (HTTP/UDS/CLI parity over the BaseHandlers spine). This child proves **the operator-facing browser surface itself**: the SPA is served by the daemon's embedded `web/dist` (`web/embed.go`), reaches a real Claude Code subprocess through `/api/sessions/:id/prompt`, renders the typed AI-SDK envelope (`text-delta`, `tool-input-*`, `tool-output-*`, `data-agh-*`, `error`, `finish`) without drift, persists transcripts across reloads, honors DESIGN.md tokens, refuses to advertise capabilities the daemon does not actually expose, and closes the truthful-UI loop. Mocks (`internal/testutil/acpmock`) are only used where the test target is a control-plane invariant (route shell, design tokens, copy vocabulary) and an LLM stream would add flake without coverage.

The CLAUDE.md / web/CLAUDE.md / DESIGN.md / COPY.md invariants this child encodes:

- **Truthful UI > plausible UI.** "Don't render controls or metrics the runtime doesn't actually support. When Paper artboards conflict with daemon truth, daemon wins. Paper governs _composition_, `DESIGN.md` governs _grammar_." (root `CLAUDE.md` Design System section). This dominates UI-10 and UI-13 and constrains every other scenario.
- **Live broadcasters publish only after durable append; reconnect/replay uses `after_seq`.** (`internal/CLAUDE.md:52`). The browser SPA must not synthesize messages locally; every visible token must come from the SSE stream and be replayable from `events.db` via `agh session events --since`.
- **AGH_WEB_API_PROXY_TARGET when the daemon is not on `:2123`.** (`web/CLAUDE.md` Critical Rules). Every isolated-lab scenario reads the bootstrap manifest and exports the proxy target before launching Vite or Playwright. Hardcoding `http://localhost:2123` is a release blocker.
- **Flat depth model, warm-dark palette, signal palette = information.** (`DESIGN.md` + `web/CLAUDE.md`). Visual regressions assert no `box-shadow`, no content gradients, and that signal colors map to documented meaning (`#E8572A` = action, `#30D158` = success, `#FF453A` = danger, `#FFD60A` = warning, `#BF5AF2` = info).
- **Backend nouns exactly. `capability`, never `recipe` / `workflow` / `procedure` / `playbook`** (`COPY.md` + `docs/_memory/glossary.md`). Static text scrape gates UI-18.
- **`claim_token` redaction is non-negotiable.** (`internal/CLAUDE.md` Security Invariants; reused in `03-acp-sessions.md` ACP-18). Raw `agh_claim_*` MUST NEVER appear in any DOM node, `aria-live` region, network-tab body, or console log.
- **Detached prompt vs request lifetime.** `internal/api/httpapi/prompt.go:104` uses `context.WithoutCancel(c.Request.Context())` — closing an SSE socket does NOT stop the LLM. The UI MUST surface explicit cancel; reload must reconnect to the same in-flight prompt.
- **Components MUST NOT import from `stores/` or `adapters/` directly** (`web/CLAUDE.md` Frontend Architecture Rules). UI-09 spot-checks this with a static import audit.
- **Cross-system imports only through public barrels** (`web/CLAUDE.md`). UI-09 spot-checks this too.

Every scenario below runs against a **real Claude Code ACP subprocess** unless explicitly marked `live: false`. Mocks via `internal/testutil/acpmock` are explicitly called out where used (route-shell rendering, fixture-replayed permission flow, copy/vocabulary scrapes) so the parity gates remain deterministic without giving up the SSE-shape assertions that need a live token stream.

## 1. Module surface — Web SPA topology

The SPA is a Bun + Vite 8 + React 19 + TanStack Router file-based app. Authoritative boundaries:

| Layer                                                                       | Responsibility (file:line refs)                                                                                                                                                                             |
| --------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Embedded static assets                                                      | `web/embed.go` — daemon serves `dist/` over `GET /` and `GET /assets/*`. Build via `make web-build`.                                                                                                        |
| Vite proxy                                                                  | `web/src/lib/vite-api-proxy-target.ts:1-22` — reads `AGH_WEB_API_PROXY_TARGET`, defaults to `http://localhost:2123`, throws on a non-URL value.                                                              |
| Root shell + global error/notfound boundaries                               | `web/src/routes/__root.tsx:9-118` (`TooltipProvider`, `Toaster`, `RootRouteErrorBoundary`, `RootRouteNotFoundBoundary`).                                                                                    |
| `_app` shell layout, sidebar, workspace onboarding gate                     | `web/src/routes/_app.tsx:18-60`. When no workspaces exist, `WorkspaceOnboarding` short-circuits the layout (`web/src/systems/workspace`). Otherwise `AppSidebar` + `<Outlet/>` mounts.                       |
| Sidebar nav (rail + workspace pills + nav rows + connection indicator)      | `web/src/components/app-sidebar.tsx:1-120+`; `web/src/components/connection-indicator.tsx` (role=status, aria-live=polite).                                                                                  |
| Home dashboard (daemon status, metrics, loading/empty/error)                | `web/src/routes/_app/index.tsx:1-203`. Uses `useHomePage()` from `web/src/hooks/routes/use-home-page.ts`.                                                                                                    |
| Permalink redirect by session id                                            | `web/src/routes/_app/session.$id.tsx:1-56`.                                                                                                                                                                 |
| Canonical session/chat route                                                | `web/src/routes/_app/agents.$name.sessions.$id.tsx:1-152`. Mounts `SessionChatRuntimeProvider` + `ChatHeader` + `SessionThread` + `SessionInspector`.                                                        |
| Chat runtime (assistant-ui)                                                 | `web/src/systems/session/components/session-chat-runtime-provider.tsx:1-46`; `web/src/systems/session/hooks/use-session-chat-runtime.ts:1-52`. Posts to `/api/sessions/${sessionId}/prompt` via `AssistantChatTransport`. |
| Streaming thread + composer                                                 | `web/src/components/assistant-ui/session-thread.tsx:1-300+` (`composer-textarea`, `composer-send-button`, `chat-view`, `composer-clear-dialog`).                                                             |
| Tool-call rendering                                                         | `web/src/systems/session/components/tool-call-card.tsx` + `tool-renderers/{bash,read,edit,write,search,generic,expanded-tool-content}.tsx`.                                                                  |
| Permission prompt                                                           | `web/src/systems/session/components/permission-prompt.tsx:1-60+` — POSTs `/api/sessions/:id/approve`.                                                                                                       |
| Session inspector (Trace / Usage / Memory / Files / Vault tabs)             | `web/src/systems/session/components/session-inspector.tsx:1-120+`.                                                                                                                                          |
| Knowledge / memory route + filtering                                        | `web/src/routes/_app/knowledge.tsx:1-60+`; `web/src/systems/knowledge/components/knowledge-list-panel.tsx`, `knowledge-detail-panel.tsx`, `knowledge-delete-dialog.tsx`; `web/src/systems/knowledge/hooks/`.   |
| Settings shell + section nav                                                | `web/src/routes/_app/settings.tsx:1-60+`; nine concrete pages under `web/src/routes/_app/settings/{general,providers,mcp-servers,memory,skills,automation,network,observability,hooks-extensions,vault}.tsx`. |
| Restart-required affordance                                                 | `web/src/systems/settings/components/settings-restart-banner.tsx:23-100+`. `data-testid="settings-page-${slug}-restart-banner"`. The "Restart now" action calls `triggerSettingsRestart`.                    |
| Network workspace shell (channels / DM / threads / details panel)           | `web/src/systems/network/components/network-workspace-shell.tsx:1-1100+`; route `web/src/routes/_app/network.tsx:1-60`.                                                                                       |
| Tasks (list / kanban / dashboard / inbox / detail / runs panel / timeline)  | `web/src/routes/_app/tasks.tsx`, `tasks.$id.tsx`, `tasks.$id.runs.$runId.tsx`, `tasks.new.tsx`; `web/src/systems/tasks/components/`. Detail-runs panel (`tasks-detail-runs-panel.tsx`) shows pending / leased / completed / failed runs and supports per-run actions. |
| Agents list                                                                 | `web/src/routes/_app/agents.$name.tsx`; `web/src/systems/agent/`.                                                                                                                                            |
| Bridges                                                                     | `web/src/routes/_app/bridges.tsx`; `web/src/systems/bridges/components/{bridge-list-panel, bridge-detail-panel, bridge-create-dialog, bridge-edit-dialog, bridge-test-delivery-dialog, bridge-empty-state}.tsx`. |
| Automation jobs / triggers / runs                                           | `web/src/routes/_app/{jobs,triggers}.tsx`; `web/src/systems/automation/components/`.                                                                                                                         |
| Vault                                                                       | `web/src/systems/vault/`; settings page `web/src/routes/_app/settings/vault.tsx`; per-session pane via `SessionInspector` Vault tab.                                                                          |
| Skills                                                                      | `web/src/routes/_app/skills.tsx`; `web/src/systems/skill/`.                                                                                                                                                  |
| Daemon status / health                                                      | `web/src/systems/daemon/`. The home dashboard reads `getDaemonStatus` (`/api/daemon/status`) and `getObserveHealth` (`/api/observe/health`).                                                                  |
| API client                                                                  | `web/src/lib/api-client.ts:1-65` (`openapi-fetch` typed against `web/src/generated/agh-openapi.d.ts`). Errors mapped via `apiErrorMessage` / `defaultApiErrorMessage`.                                        |
| Toaster surface                                                             | `web/src/routes/__root.tsx:33` (`<Toaster/>`); `sonner.toast.error` used in `web/src/systems/session/components/permission-prompt.tsx:30`, `web/src/systems/session/hooks/use-session-create-dialog.ts:185`, `web/src/systems/workspace/hooks/use-workspace-setup-content.ts:71`. |
| Connection state semantics                                                  | `web/src/components/connection-indicator.tsx:38-52` — `role=status`, `aria-live=polite`, `data-status={connected|disconnected|reconnecting}` chip.                                                            |

The CLI/back-end terms used by the SPA come from the typed contract barrel `web/src/generated/agh-openapi.d.ts` (single source of truth for every operation/payload/parameter the SPA can call). Any test that asserts a payload shape MUST import from this generated module — not from a hand-written shim.

The Playwright lane runner is `web/e2e/fixtures/runtime.ts:1-615` (builds the daemon with `go build -o … ./cmd/agh`, spawns `agh daemon start --foreground`, isolates `AGH_HOME`, reserves a free TCP port, and exposes `runtime.url(pathname)` + `requestJSON` + `requestOperatorJSON`). The daemon binary path is overridable via `AGH_TEST_DAEMON_BIN`. Mock-agent fixtures plug in via `runtimeOptions.seed.mockAgents[]` (`web/e2e/fixtures/runtime-seed.ts`) using `internal/testutil/acpmock` JSON fixtures.

## 2. Existing coverage — do NOT duplicate

Tests already in tree the real-LLM web QA must NOT replicate:

- Vitest unit / route tests under `web/src/routes/_app/-*.test.tsx`, every system's `-*.test.tsx`, every adapter's `-api.test.ts`, every hook's `-page.test.tsx` and `-actions.test.tsx`. These prove component contracts in isolation.
- `web/src/lib/agent-authored-context-contract.test.ts`, `agent-authored-context-no-ui.test.ts`, `daemon-api-contract.test.ts`, `settings-api-contract.test.ts`, `vite-api-proxy-target.test.ts` — typed-contract assertions against the generated OpenAPI module.
- Playwright specs already in `web/e2e/`:
  - `harness-smoke.spec.ts` — Playwright launches the daemon in launch mode and serves the SPA over the embedded asset path.
  - `session-onboarding.spec.ts`, `session-provider-override.spec.ts` — session create / streaming / permission approve / reload / stop / resume — but driven by `internal/testutil/acpmock` fixtures, **not** a real LLM.
  - `settings.spec.ts`, `settings-transport.spec.ts` — settings shell navigation, restart banner, transport parity strip.
  - `network.spec.ts`, `automation.spec.ts`, `bridges.spec.ts`, `tasks.spec.ts`, `tasks-coordinator-handoff.spec.ts`, `combined-flows.spec.ts`, `storybook-bootstrap.spec.ts` — feature-shell regression suites against mocks.

The gap real-LLM web scenarios MUST close: every existing Playwright spec drives the daemon against `acpmock` JSON fixtures. **None spawn a real Claude Code subprocess and prove the SPA renders the live SSE token stream incrementally, persists it across reloads, cancels mid-token via the explicit cancel control, redacts `claim_token`, and stays within the truthful-UI invariant.** None statically scrape the rendered DOM for COPY.md vocabulary violations or assert `aria-live` discipline on the streaming region.

## 3. Gaps the real-scenario web lane must close

1. **Real SSE token rendering**: tokens appear in the DOM incrementally, in the same order as `events.db` rows, with a visible streaming indicator that flips off only after `finish` (UI-02, UI-03).
2. **Reload mid-stream**: closing the tab does NOT cancel the prompt (detached lifetime); reopening the route reconnects the SSE and tokens continue from `after_seq` without duplicates (UI-03, UI-15).
3. **Explicit cancel mid-token**: clicking the Stop control inside the chat triggers `POST /api/sessions/:id/prompt/cancel`; UI flips to a "cancelled" state within ≤2s; daemon transcript ends in `prompt_cancelled` / `stop_reason: "canceled"` (UI-04).
4. **Tool calls**: `tool-input-start` → `tool-input-available` → `tool-output-available` SSE frames render as a collapsible card; sensitive bytes (`agh_claim_*`, vault values) are NEVER inlined (UI-05, UI-17).
5. **Memories list**: list/grid views, type/scope filters, recall preview, delete, and consolidation status indicator each read from real daemon state (UI-06).
6. **Hot-apply settings vs restart-required**: a known hot-apply key reflects without reload; a known restart-required key surfaces the banner with a one-click "Restart now" affordance and the daemon does in fact restart (UI-07).
7. **Bridges**: list / detail / test-delivery roundtrip via `POST /api/bridges/:id/test-delivery` (UI-08).
8. **Sessions / lineage tree**: parent → child sessions visible; click child opens the canonical chat route (UI-09).
9. **Task runs queue**: `pending` / `leased` / `completed` / `failed` lanes; per-run lease TTL; admin-cancel; DOM never embeds `claim_token` raw text (UI-10).
10. **Truthful UI capability check**: a control whose backing capability is OFF in `agh status` does NOT render (UI-11).
11. **Accessibility**: keyboard reaches every interactive control along the streaming chat; visible focus rings; `aria-live` for SSE token regions and connection indicator; color contrast ≥ DESIGN.md tokens (UI-12).
12. **Responsive**: 1440x900 / 1024x768 / 768x1024 / 390x844 — no overflow, no clipped content; sidebar collapse works (UI-13).
13. **Theme adherence**: every page passes the DESIGN.md gate — flat depth (no `box-shadow`), warm-dark canvas (`--color-canvas`/`--color-canvas-deep`), signal palette only where signal is intended (UI-14).
14. **Tab-visibility SSE reconnect**: tab hidden 60s, foregrounded — UI catches up via `after_seq`; no duplicates (UI-15).
15. **Error toasts**: every typed error path from `BaseHandlers` maps to a user-facing toast with action affordance; raw `error.message` from openapi-fetch is never rendered untyped (UI-16).
16. **`claim_token` redaction**: deep DOM scan never matches `/agh_claim_[A-Za-z0-9_-]+/` across any rendered route (UI-17).
17. **COPY.md vocabulary**: every visible string passes the canonical-term gate (no `recipe` / `workflow` / `playbook` / `procedure` for current AGH artifacts) (UI-18).
18. **DX-cliff catch**: a fake auth method or any orphan control left in any settings panel that the daemon doesn't actually implement — must NOT exist; static audit fails the run (UI-11 + UI-18 cross-check).
19. **Worktree-isolated proxy**: when the daemon is on a non-default port, the SPA reads `AGH_WEB_API_PROXY_TARGET` from `bootstrap.env` and reaches `/api/...` correctly (UI-01 sub-assertion).

## 4. Operating model — provider matrix and bootstrap

Same template as `03-acp-sessions` and `11-api-cli-parity`:

- **`real-claude-code`** (default): real Claude Code ACP subprocess for any scenario that drives a streaming prompt or tool turn. Driver: `internal/config/provider.go:165-173` (`npx -y @agentclientprotocol/claude-agent-acp@latest`).
- **`mock-acp`** (used only for: route shell loading/empty/error, design-token / color-contrast scans, copy/vocabulary scrapes, accessibility static gates): `internal/testutil/acpmock` JSON fixtures plugged via `runtimeOptions.seed.mockAgents[]` (`web/e2e/fixtures/runtime-seed.ts`). These never exercise an LLM token stream.

Bootstrap and isolation discipline (mandatory for every scenario):

- One isolated `AGH_HOME`, daemon HTTP port, UDS socket path, `tmux-bridge` socket, and `PROVIDER_HOME`/`PROVIDER_CODEX_HOME` per scenario (per `agh-worktree-isolation` skill and `agh-qa-bootstrap`).
- **`AGH_WEB_API_PROXY_TARGET` exported from `bootstrap.env`** before launching either Vite (`make web-dev`) or Playwright (`make test-e2e-web`). Hardcoding `http://localhost:2123` is a blocker (`web/CLAUDE.md` Critical Rules + `web/src/lib/vite-api-proxy-target.ts`).
- Bound-secret, brokered, and explicitly isolated-home auth staged into
  `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`; `native_cli` providers with
  `home_policy=operator` intentionally use the operator `HOME` / native login
  state unless the scenario explicitly validates isolated provider-home
  behavior.
- Sequential config writes only when targeting the same provider home.
- Browser context: Playwright Chromium headless by default; `PLAYWRIGHT_HEADFUL=1` for local debugging only; `workers: 1` (per `web/playwright.config.ts:14`); per-scenario timeout 90s with retries 0 locally / 1 in CI; trace/screenshot/video off by default — turn on per-scenario via `test.use({ trace: "retain-on-failure", screenshot: "only-on-failure" })` when capturing evidence.

Per-scenario evidence layout under `.artifacts/qa/<run-id>/ui-XX/`:

- `ui-XX-report.md` (Worked / Failed / Blocked / Follow-up).
- `ui-XX-summary.json` (machine-readable: pass/fail counts, screenshot paths, network HAR ref).
- `ui-XX-events.json` (events.db rows scoped to the scenario window via `agh session events <id> --since <ts> -o jsonl`).
- `ui-XX-output.log` (combined daemon stdout/stderr from `runtime.paths.daemonLog`).
- `ui-XX-screenshots/` (named keys per assertion).
- `ui-XX-network.har` (full network trace via `context.tracing.start({ snapshots: true, sources: false })` opt-in).
- `ui-XX-axe.json` (axe-core violations dump, when accessibility is asserted — UI-12).
- `ui-XX-dom-strings.json` (text-content scrape used by UI-18 vocabulary gate).

## 5. Preconditions (apply to every scenario)

- Fresh QA bootstrap; `bootstrap-manifest.json` saved and `bootstrap.env` exported. `AGH_HOME`, daemon HTTP port, UDS socket, `PROVIDER_HOME`/`PROVIDER_CODEX_HOME`, `AGH_WEB_API_PROXY_TARGET` all set.
- `make verify` is green on the SUT branch (per the Critical Rules). In particular `make web-build` and `make web-typecheck` MUST be green so the embedded `dist/` is up to date.
- Daemon running: `agh status -o json` reports `status="running"`.
- For `real-claude-code` scenarios: direct `claude` auth comes from the
  effective Claude home for the lane (operator `HOME` by default; isolated
  `PROVIDER_HOME` only for explicit isolated-home scenarios); `agh provider
  show claude` reports the expected ACP command.
- Workspace seed: `$LAB/workspace/` with a `README.md` (≥3 paragraphs), `src/file_a.go`, `src/file_b.go`, and a `generated_long_file.txt` (~2MB) for streaming-volume scenarios.
- Playwright test binary built with the daemon binary path; `AGH_TEST_DAEMON_BIN` set when running against a pre-built binary.
- Browser preferences neutral: no extensions, no system zoom override, system locale `en-US`, prefers-reduced-motion `no-preference` for default scenarios; UI-12 explicitly toggles `prefers-reduced-motion: reduce`.

## 6. Cleanup (applies to every scenario)

- `runtime.dispose()` in the Playwright fixture (kills the daemon child via SIGINT then SIGKILL with a 10s grace per `web/e2e/fixtures/runtime.ts:414-427`).
- Verify no orphan subprocesses (`pgrep -f claude-agent-acp` returns empty before tearing down the worktree).
- Archive evidence directory before deleting the temp `AGH_HOME`.
- Run `goleak.VerifyNone` equivalent on the daemon shutdown path through `agh daemon stop`-style supervised mode when the scenario explicitly enables it.
- Tear down the worktree only after evidence artifacts are written.

## 7. Mandatory scenarios

### UI-01 — Cold-boot home dashboard with daemon running, then offline-state, then load error

```yaml qa-scenario
id: ui-01-home-states
title: Home dashboard renders loading → connected with daemon metrics; offline state surfaces a Disconnected card; backend error surfaces an Empty error card
theme: web.home
coverage:
  primary:
    - web.home.connected
    - web.home.disconnected
    - web.home.error
  secondary:
    - web.layout._app
    - web.connection_indicator
risk: high
live: false
provider: mock-acp
preconditions:
  - bootstrap-manifest written; daemon launched in launch mode by Playwright runtime (`web/e2e/fixtures/runtime.ts:112-173`)
  - SPA served from embedded `web/dist` at `runtime.url("/")`
docs_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/DESIGN.md
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/index.tsx:20-167
  - /Users/pedronauck/Dev/compozy/agh/web/src/components/connection-indicator.tsx:23-52
  - /Users/pedronauck/Dev/compozy/agh/web/src/lib/vite-api-proxy-target.ts:1-22
steps:
  - |
    Subtest A — connected:
      await page.goto(runtime.url("/"));
      await expect(page.getByTestId("home-shell")).toBeVisible();
      await expect(page.getByTestId("home-loading")).toBeVisible();
      await expect(page.getByTestId("home-loading")).toBeHidden({ timeout: 10_000 });
      await expect(page.getByTestId("home-page-title")).toHaveText("Home");
      await expect(page.getByTestId("home-section-daemon")).toBeVisible();
      await expect(page.getByTestId("home-daemon-card")).toBeVisible();
      await expect(page.getByTestId("home-daemon-status-dot")).toHaveAttribute("data-status", /running|ready|connected/);
      await expect(page.getByTestId("home-connection-indicator")).toHaveAttribute("data-status", "connected");
      await expect(page.getByTestId("home-metric-active-sessions")).toBeVisible();
      await expect(page.getByTestId("home-metric-workspaces")).toBeVisible();
      await expect(page.getByTestId("home-metric-agents")).toBeVisible();
      await expect(page.getByTestId("home-metric-uptime")).toBeVisible();
  - |
    Subtest B — disconnected (daemon stopped mid-session):
      await runtime.dispose(); // SIGINT the daemon
      await expect.poll(() => page.getByTestId("home-connection-indicator").getAttribute("data-status"), { timeout: 30_000 }).toBe("disconnected");
      await expect(page.getByTestId("home-daemon-disconnected")).toBeVisible();
      await expect(page.getByTestId("home-daemon-disconnected-indicator")).toHaveAttribute("data-status", "disconnected");
  - |
    Subtest C — load error (daemon returns 5xx for /api/daemon/status):
      Use Playwright `page.route("**/api/daemon/status", route => route.fulfill({ status: 500, body: JSON.stringify({ error: "internal" }) }))` BEFORE the initial navigation; reload.
      await expect(page.getByTestId("home-error")).toBeVisible();
      await expect(page.getByTestId("home-error")).toContainText(/Unable to load dashboard/i);
expected:
  - Loading skeleton (`home-loading` + `home-daemon-skeleton` + `home-metric-skeleton`) is visible during the initial fetch and disappears within 10s on a healthy daemon.
  - Connected pill is `tone=success`; daemon card carries `data-status` derived from real `getDaemonStatus` payload (NOT a placeholder).
  - Vite proxy target was the value from `bootstrap.env` (`AGH_WEB_API_PROXY_TARGET`), proven by inspecting `runtime.url("/")` → host:port and the network HAR — every `/api/*` request goes there.
  - Disconnected card uses the `Empty` primitive with `ServerOff` icon and the Disconnected indicator (`web/src/routes/_app/index.tsx:122-138`).
  - Error card uses the `Empty` primitive with `AlertTriangle` icon and the daemon error message (`web/src/routes/_app/index.tsx:48-61`).
evidence:
  - ui-01-screenshots/{connected,disconnected,error}.png
  - ui-01-network.har showing /api/daemon/status round trips
  - ui-01-summary.json
failure_signatures:
  - Loading skeleton shown forever — `useHomePage` query never resolves (likely Vite proxy misconfigured against the wrong port).
  - Disconnected indicator shows `connected` after daemon SIGINT — query has no `staleTime`/refetch loop.
  - Error state renders raw error JSON — `apiErrorMessage` regression in `web/src/lib/api-client.ts`.
cleanup:
  - runtime.dispose() (idempotent).
```

### UI-02 — Real Claude Code chat: send prompt, SSE tokens render incrementally, final message persists across reload

```yaml qa-scenario
id: ui-02-chat-streaming-roundtrip
title: Operator sends a prompt to a real Claude Code session; SSE text-deltas render incrementally in the DOM; final message persists; reloading the route shows the same conversation
theme: web.chat.live
coverage:
  primary:
    - web.chat.streaming
    - web.chat.persistence
    - sse.typed_envelope
  secondary:
    - api.parity.http
    - transcript.canonical
risk: high
live: true
provider: real-claude-code
preconditions:
  - ACP-01 preconditions (Claude Code subprocess reachable; direct `claude`
    auth resolved from the effective Claude home for the lane)
  - SPA served from runtime.url("/"); workspace exists (use `Use this workspace globally` if onboarding fires)
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/agents.$name.sessions.$id.tsx:19-152
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/session-chat-runtime-provider.tsx:1-46
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/hooks/use-session-chat-runtime.ts:1-52
  - /Users/pedronauck/Dev/compozy/agh/web/src/components/assistant-ui/session-thread.tsx:140-300
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/prompt.go:90-156
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/prompt.go:251-580
steps:
  - |
    await page.goto(runtime.url("/"));
    if (await page.getByTestId("workspace-onboarding").isVisible()) {
      await page.getByTestId("workspace-use-global").click();
      await expect(page.getByTestId("workspace-onboarding")).toBeHidden();
    }
    await expect(page.getByTestId("app-sidebar")).toBeVisible();
    await page.getByTestId("nav-agents").click();
    await page.getByRole("link", { name: /claude/i }).first().click();
  - |
    const createResponsePromise = page.waitForResponse(r =>
      r.request().method() === "POST" && r.url().endsWith("/api/sessions"));
    await page.getByRole("button", { name: /new session|start session/i }).click();
    await page.getByTestId("session-create-dialog-submit").click();
    const createResp = await createResponsePromise;
    expect(createResp.ok()).toBeTruthy();
    const { session: { id: sessionId } } = await createResp.json();
  - |
    await expect(page.getByTestId("chat-header")).toBeVisible();
    await expect(page.getByTestId("composer-textarea")).toBeVisible();
    await page.getByTestId("composer-textarea").fill("Read README.md and tell me the title in one sentence.");

    const promptResponsePromise = page.waitForResponse(r =>
      r.url().endsWith(`/api/sessions/${sessionId}/prompt`) && r.request().method() === "POST");
    await page.getByTestId("composer-send-button").click();
    const promptResp = await promptResponsePromise;
    expect(promptResp.headers()["content-type"]).toContain("text/event-stream");
  - |
    // Assert incremental rendering — sample chat-view text length over time and prove monotonic growth.
    const samples: number[] = [];
    for (let i = 0; i < 6; i++) {
      const txt = await page.getByTestId("chat-view").textContent();
      samples.push(txt?.length ?? 0);
      await page.waitForTimeout(500);
    }
    expect(samples.at(-1)!).toBeGreaterThan(samples[0]);
    // Strictly monotonic except for trailing stable samples.
    for (let i = 1; i < samples.length; i++) expect(samples[i]).toBeGreaterThanOrEqual(samples[i-1]);
  - |
    // Wait for stream-complete: assistant message present, processing indicator gone.
    await expect(page.getByTestId("processing-indicator")).toBeHidden({ timeout: 60_000 });
    const finalText = await page.getByTestId("chat-view").textContent();
    expect(finalText).toBeTruthy();
  - |
    // Persistence proof: events.db has rows for this session window.
    const events = await runtime.requestOperatorJSON<unknown[]>(
      `/api/sessions/${sessionId}/events?limit=200`);
    expect(events.length).toBeGreaterThan(0);
  - |
    // Reload route — transcript continues to render.
    const sessionPath = new URL(page.url()).pathname;
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect.poll(() => new URL(page.url()).pathname).toBe(sessionPath);
    await expect(page.getByTestId("chat-view")).toContainText("Read README.md");
expected:
  - `composer-send-button` POSTs to `/api/sessions/:id/prompt` with `Content-Type: application/json`; response is `text/event-stream`.
  - DOM token length grows monotonically as deltas arrive — proves no buffer-then-flush regression in `assistant-ui`.
  - Final chat view contains the model reply; `processing-indicator` (`web/src/components/assistant-ui/session-thread.tsx:281` proximity) is hidden after `finish`.
  - `events.db` contains ordered rows from this scenario window with schema `agh.session.event.v1`.
  - After reload, the chat view re-mounts the persisted transcript via `sessionTranscriptOptions` (`web/src/systems/session/lib/query-options.ts`); content matches the pre-reload final text.
evidence:
  - ui-02-network.har (POST /prompt SSE response stream)
  - ui-02-screenshots/{thinking,streaming,after-finish,after-reload}.png
  - events.json from `agh session events $sessionId -o jsonl`
  - ui-02-summary.json (token-length samples)
failure_signatures:
  - Single full-text snap at the end with no monotonic growth — the SPA buffers the entire message instead of rendering deltas (regression of `useChatRuntime` wiring at `web/src/systems/session/hooks/use-session-chat-runtime.ts:36-49`).
  - SSE response served as `application/json` — wrong handler path used (regression of `internal/api/httpapi/prompt.go:251-580`).
  - After reload the chat-view is empty — `sessionTranscriptOptions` is not invalidated/refetched on mount, or the daemon dropped the events.
cleanup:
  - await page.getByTestId("delete-button").click(); confirm; assert empty list.
  - runtime.dispose().
```

### UI-03 — Reload mid-stream: detached prompt continues, SSE reconnect via `after_seq` shows no duplicates

```yaml qa-scenario
id: ui-03-reload-mid-stream
title: Reloading the chat route while tokens are streaming does NOT cancel the prompt; reconnect resumes from after_seq with no duplicate tokens
theme: web.chat.detached_lifetime
coverage:
  primary:
    - web.chat.reload_mid_stream
    - sse.replay
    - sse.last_event_id
  secondary:
    - detached_lifetime
    - transcript.canonical
risk: high
live: true
provider: real-claude-code
preconditions:
  - UI-02 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/prompt.go:104
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/session_stream.go:69-100
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/handlers.go:521
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/hooks/use-session-chat-runtime.ts:34-49
steps:
  - Create session and start a prompt that produces ≥30s of output (e.g. "Recursively summarize every file under src/ and produce a 200-line report").
  - After ~3s of streaming, capture the partial chat-view text length L1 and the last event id L_id from the SSE network frames (Playwright network event listener).
  - page.reload({ waitUntil: "domcontentloaded" }).
  - Wait for the chat-view to remount; assert the partial text is at least L1 (no regression to empty), then assert it continues to grow.
  - Wait for `finish`; capture final text length L2 and the chat-view text C2.
  - Compare with `agh session transcript $S -o json`: full transcript text equals C2 modulo whitespace; no duplicate sentences inside a 200-char rolling window (regex sniff for the same 80-char substring appearing twice).
expected:
  - Step 2: SSE TCP connection from the closing tab is observable as closed in the daemon access log; daemon log records `prompt active, request context detached` (or equivalent) — proves `prompt.go:104` (`context.WithoutCancel`) detached the prompt.
  - Step 3-4: After remount, the SPA reissues `GET /api/sessions/:id/stream` with `Last-Event-ID` derived from the persisted transcript; only events with `sequence > L_id` are streamed (`session_stream.go:77-100`).
  - Step 5: Final transcript reconstructed from `events.db` matches DOM C2 byte-for-byte (modulo whitespace).
  - Step 6: Duplicate-window scan finds no repeated 80-char substring with offset ≥ 200 chars apart — no replay duplicate.
evidence:
  - ui-03-network.har showing two SSE connections (pre-reload, post-reload) with identical session id but distinct connection ids
  - ui-03-screenshots/{pre-reload,post-reload-mid,after-finish}.png
  - ui-03-events-diff.txt: full events.json vs DOM scrape diff (must be empty modulo whitespace)
failure_signatures:
  - Tokens stop after reload and never resume — daemon followed request context cancellation (regression of detached-lifetime invariant).
  - Reload replays text from sequence 0 — `Last-Event-ID` not exposed via CORS or not honored by the SPA.
  - Same sentence duplicated across pre/post boundary — broken `pollAndStreamSessionEvents` cursor.
cleanup:
  - delete session; runtime.dispose().
```

### UI-04 — Cancel a streaming prompt mid-token: UI flips to cancelled state ≤2s; daemon transcript shows `prompt_cancelled`

```yaml qa-scenario
id: ui-04-cancel-mid-token
title: Clicking Stop mid-token cancels the active prompt within ≤2s; UI reflects cancellation; events.db ends with stop_reason "canceled"
theme: web.chat.cancel
coverage:
  primary:
    - web.chat.cancel
    - acp.cancel
  secondary:
    - sse.error_finish
    - transcript.cancelled
risk: high
live: true
provider: real-claude-code
preconditions:
  - UI-02 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/chat-header.tsx:163
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/sessions.go:43-50
  - /Users/pedronauck/Dev/compozy/agh/internal/acp/client.go:594-610
steps:
  - Start a long prompt as in UI-03.
  - After 3s of streaming, click `getByTestId("stop-button")` (the chat header Stop control). Capture wall-clock t_click.
  - Watch for the chat-view to flip into a cancelled state (`processing-indicator` hidden, no further deltas, an explicit "stopped" / "canceled" affordance or pill visible — pull the canonical text from `chat-header.tsx`).
  - Capture wall-clock t_state when stop_reason text becomes visible. Assert (t_state - t_click) ≤ 2.0s.
  - Issue follow-up prompt; assert it works (session is still alive — Stop only cancels the in-flight prompt, not the session). Note: when the chat header Stop action is wired to the broader session stop path it MAY end the session; the scenario asserts whichever invariant the implementation guarantees and documents that contract here.
  - Inspect `agh session events $S --type agent_event -o json | jq '.[-3:]'` and confirm at least one terminal event carries `stop_reason == "canceled"` (or maps via `aiSDKFinishReason("canceled")` to `"other"` per `internal/api/httpapi/prompt.go:569-580`).
expected:
  - Stop control is reachable via keyboard (Tab order); Enter/Space activates it.
  - Network HAR shows `POST /api/sessions/:id/prompt/cancel` returning 2xx within ≤500ms.
  - Final SSE frame carries `finish` with the cancel-mapped reason; `processing-indicator` hidden.
  - events.db tail row has `stop_reason="canceled"`.
evidence:
  - ui-04-network.har (cancel POST + final SSE finish)
  - ui-04-screenshots/{streaming,after-cancel}.png
  - ui-04-events-tail.json (last 5 events)
failure_signatures:
  - UI continues to render deltas for >2s after click — cancel POST not issued, or daemon `Cancel` notification not delivered (regression of `acp/client.go:594-610`).
  - events.db ends with `stop_reason="stop"` instead of `"canceled"` — broken classification.
  - Stop control unreachable by keyboard — focus order regression in chat header.
cleanup:
  - delete session; runtime.dispose().
```

### UI-05 — Multi-turn conversation with tool calls: tool card collapsible, sensitive bytes redacted

```yaml qa-scenario
id: ui-05-tool-card-and-redaction
title: Tool calls render as collapsible cards with input/output; sensitive tokens (claim_token, vault values) are not visible in the DOM at any time
theme: web.chat.tool_calls
coverage:
  primary:
    - web.chat.tool_card
    - security.claim_token_redaction
  secondary:
    - sse.typed_envelope
    - transcript.canonical
risk: high
live: true
provider: real-claude-code
preconditions:
  - UI-02 preconditions
  - workspace seed includes `README.md` and `src/file_a.go` (real files for the read tool)
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/tool-call-card.tsx
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/tool-renderers/{bash,read,edit,write,search,generic,expanded-tool-content}.tsx
  - /Users/pedronauck/Dev/compozy/agh/internal/transcript/transcript.go:239-318
steps:
  - Create a session, send: "Read README.md, then read src/file_a.go, then write a one-paragraph summary to summary.md."
  - Wait for at least three tool-call cards to render (`getByTestId("tool-call-card")`).
  - For each card: verify the trigger (`tool-card-trigger`) carries a label test id (e.g. `read`, `write`, `bash`); click to expand → assert `tool-card-expanded` becomes visible; collapse and re-expand; assert state is preserved across collapse/expand.
  - Snapshot the full DOM textContent. Run `match(/agh_claim_[A-Za-z0-9_-]+/g)` and assert null match. Run `match(/sk-(ant|prod)-[A-Za-z0-9_-]+/g)` and assert null match (scopes any provider key that might leak through prompt metadata).
  - Multi-turn proof: after the first turn, the operator sends a follow-up "Now show me the summary you wrote." The agent reads summary.md; the new tool card appears; the older cards remain in their original positions (no re-order).
expected:
  - Each tool card emits the SSE sequence `tool-input-start` → `tool-input-available` → `tool-output-available` (cross-check with `agh session events $S --type tool_call,tool_result -o json`).
  - Cards are keyboard-toggleable: `aria-expanded` flips between true/false.
  - DOM scan for `agh_claim_*` returns 0 matches across the entire session lifecycle (rendered or hidden subtrees).
  - Cards preserve order across follow-up turns (turn change does not reorder past tool cards).
evidence:
  - ui-05-screenshots/{collapsed,expanded,after-followup}.png
  - ui-05-dom-needle-scan.json (counts per regex; all 0)
  - tool_calls.json from `agh session events`
failure_signatures:
  - DOM contains a raw `agh_claim_` token — release blocker (CLAUDE.md "claim_token redaction is non-negotiable"). Cross-link to ACP-18.
  - Tool card always expanded with no collapse affordance — regression of `tool-card-trigger` toggle.
  - Tool input rendered as JSON.stringify with ad-hoc whitespace — regression of `tool-input-available` decoder (`web/src/systems/session/lib/message-parts.ts`).
cleanup:
  - delete session; remove summary.md; runtime.dispose().
```

### UI-06 — Knowledge (memories) browse / filter / preview / delete / consolidation

```yaml qa-scenario
id: ui-06-knowledge-page
title: Knowledge route lists memories with type/scope filters, preview content, supports delete with confirmation, and surfaces consolidation status
theme: web.knowledge
coverage:
  primary:
    - web.knowledge.list
    - web.knowledge.filter
    - web.knowledge.delete
    - web.knowledge.consolidation
  secondary:
    - api.parity.http
risk: medium
live: false
provider: mock-acp
preconditions:
  - Seed memory store with ≥3 memories of mixed types (`session`, `workspace`, `global`) and ≥2 distinct kinds (`snapshot`, `transcript`); use `runtime.requestOperatorJSON` to POST seeds via `/api/memory` (UDS) ahead of UI navigation.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/knowledge.tsx:1-60
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/knowledge/components/knowledge-list-panel.tsx:1-180
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/knowledge/components/knowledge-detail-panel.tsx
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/knowledge/components/knowledge-delete-dialog.tsx
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/knowledge/hooks/use-knowledge-actions.ts
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/settings/memory.tsx:193 (consolidate trigger)
steps:
  - await page.goto(runtime.url("/knowledge"));
  - await expect(page.getByTestId("knowledge-shell")).toBeVisible();
  - await expect(page.getByTestId("knowledge-list-panel")).toBeVisible();
  - Filter by ALL → GLOBAL → WORKSPACE via the pill group (`tab-pills` / `tab-all` / `tab-global` / `tab-workspace`). Assert the visible groups (`knowledge-group-${scope}`) match the active filter.
  - await page.getByTestId("knowledge-search-input").fill("session-canary");
  - Click a memory item (`memory-item-${key}`) → detail panel shows kind, scope, preview, raw content; type/scope chips render with the documented signal palette.
  - Click delete → `knowledge-delete-dialog` visible → cancel → assert memory still present → re-open → confirm → assert memory removed and toast ("Memory deleted") appears.
  - Trigger consolidation: navigate to `/settings/memory`, click `settings-page-memory-consolidate`. Wait for `settings-page-memory-action-message` to surface a result. Re-navigate to `/knowledge`; expect the consolidation status indicator to reflect a recent run (`settings-page-memory-last-consolidated` formatted timestamp updated).
expected:
  - Filter pills produce stable url-state via TanStack Router search params.
  - Detail panel renders content unmodified (no markdown injection).
  - Delete dialog uses Cancel + Confirm Destructive primitives (DESIGN.md `danger` tone for the confirm).
  - Consolidation result appears in a toast and updates the memory page summary.
evidence:
  - ui-06-screenshots/{list,filter-global,detail,delete-dialog,after-delete,consolidate-result}.png
  - ui-06-network.har (DELETE /api/memory/{id}, POST /api/memory/consolidate)
failure_signatures:
  - Filter changes do not produce search-param updates — TanStack Router state regression.
  - Delete confirms without dialog — destructive guard missing (CLAUDE.md UX rule).
  - Consolidation succeeds without UI feedback — toast wiring missing.
cleanup:
  - delete remaining seed memories via `/api/memory/{id}`; runtime.dispose().
```

### UI-07 — Settings: hot-apply key vs restart-required key (with one-click "Restart now")

```yaml qa-scenario
id: ui-07-settings-hot-apply-vs-restart
title: A hot-apply settings key reflects without reload; a restart-required key surfaces the SettingsRestartBanner with a one-click action that actually restarts the daemon
theme: web.settings
coverage:
  primary:
    - web.settings.hot_apply
    - web.settings.restart_required
  secondary:
    - web.settings.save_bar
risk: high
live: false
provider: mock-acp
preconditions:
  - daemon running with default config; SPA on /settings/general
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/settings/general.tsx:60-300
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/settings/components/settings-restart-banner.tsx:23-100
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/settings/components/settings-page-actions.tsx
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/settings/components/settings-save-bar.tsx
steps:
  - await page.goto(runtime.url("/settings/general"));
  - await expect(page.getByTestId("settings-shell")).toBeVisible();
  - |
    Subtest A — hot-apply (e.g. session timeout knob, identified as hot-apply by `settings-api`):
      const newValue = await nextSessionTimeoutValue(page.getByTestId("settings-page-general-session-timeout-input"));
      await page.getByTestId("settings-page-general-session-timeout-input").fill(newValue);
      await page.getByRole("button", { name: /save/i }).click();
      // Hot-apply MUST NOT show the restart banner.
      await expect(page.getByTestId("settings-page-general-restart-banner")).toBeHidden({ timeout: 2_000 });
      // Verify via UDS: GET /api/settings/general now reports the new value with no daemon restart required.
      const cur = await runtime.requestOperatorJSON("/api/settings/general");
      expect(cur.session_timeout).toEqual(parsed(newValue));
  - |
    Subtest B — restart-required (e.g. default sandbox provider — flagged as restart_required by the spec; pick whichever knob the spec marks):
      await page.getByTestId("settings-page-general-default-provider-input").click();
      // pick a different provider, save
      await page.getByRole("button", { name: /save/i }).click();
      await expect(page.getByTestId("settings-page-general-restart-banner")).toBeVisible();
      await expect(page.getByTestId("settings-page-general-restart-banner-message")).toContainText(/restart/i);
      // One-click "Restart now" affordance — extract from `settings-restart-banner.tsx:83+`
      await page.getByRole("button", { name: /restart/i }).click();
      // Banner enters polling state, then resolves.
      await expect.poll(async () =>
        page.getByTestId("settings-page-general-restart-banner-message").textContent(),
        { timeout: 60_000 }
      ).toMatch(/Daemon restarted successfully|Restart succeeded/);
expected:
  - Hot-apply: no restart banner; UDS reports the new value; daemon process did NOT restart (PID unchanged via `runtime.process.pid`).
  - Restart-required: banner visible with `tone=warning` while polling, then `tone=success` on completion; daemon PID DID change (or `agh status` reports a fresh started_at).
  - The "Restart now" button is keyboard-reachable; pressing Enter triggers the same flow.
evidence:
  - ui-07-screenshots/{hot-apply-after-save,restart-banner,restart-success}.png
  - ui-07-network.har (PATCH /api/settings/..., POST /api/settings/restart)
  - ui-07-pid-trace.txt (daemon PID before/after each subtest)
failure_signatures:
  - Hot-apply triggers a restart banner — settings-api classifier regression.
  - Restart banner success tone never surfaces — restart-poll loop regression in `useSettingsRestart` (`web/src/hooks/routes/use-settings-general-page.ts`).
  - Daemon PID unchanged after restart-required save — restart did not actually run; banner lies (truthful UI violation).
cleanup:
  - revert all changes; runtime.dispose().
```

### UI-08 — Bridges: list / detail / test-delivery roundtrip with explicit error states

```yaml qa-scenario
id: ui-08-bridges-test-delivery
title: Bridges page lists bridges, opens detail, and the test-delivery dialog reaches the daemon and surfaces success/failure inline; failure shows actionable error citing the offending field
theme: web.bridges
coverage:
  primary:
    - web.bridges.list
    - web.bridges.test_delivery
  secondary:
    - api.parity.http
risk: medium
live: false
provider: mock-acp
preconditions:
  - Seed at least one bridge config via `runtime.requestOperatorJSON("/api/bridges", { method: "POST", ... })` ahead of navigation; pick a bridge type whose adapter is available in the test build (per `internal/transport`).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/bridges/components/bridge-list-panel.tsx
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/bridges/components/bridge-detail-panel.tsx
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/bridges/components/bridge-test-delivery-dialog.tsx
steps:
  - await page.goto(runtime.url("/bridges"));
  - await expect(page.getByTestId(/bridge-list/i)).toBeVisible();
  - Click the seeded bridge to open the detail panel.
  - Click the "Test delivery" button → dialog opens with a payload editor.
  - Submit a malformed payload (e.g. missing required `target` field) → assert dialog shows an inline error citing the missing field name (NOT a raw 400 stack trace).
  - Submit a valid payload → assert success indicator (success-toned chip + toast).
  - Verify daemon-side: bridge runs increment by 1 (via `runtime.requestOperatorJSON("/api/bridges/:id/runs")`).
expected:
  - Validation errors use copy from the daemon's error envelope translated through `apiErrorMessage`.
  - Success/failure paint within ≤2s of the POST response.
  - Toast affordance includes a "View runs" or equivalent action that deep-links into the runs panel.
evidence:
  - ui-08-screenshots/{list,detail,test-delivery-dialog,error-state,success-state}.png
  - ui-08-network.har (POST /api/bridges/:id/test-delivery)
failure_signatures:
  - Error toast renders raw JSON — apiErrorMessage regression.
  - Success toast appears even when the daemon returned 4xx — request inference regression in the bridges-api adapter.
cleanup:
  - delete seeded bridge; runtime.dispose().
```

### UI-09 — Sessions / lineage tree visible; click child opens its transcript

```yaml qa-scenario
id: ui-09-sessions-lineage
title: Parent session that spawned a child shows the child in a lineage view; clicking the child navigates to its canonical chat route and the SessionInspector reflects parent
theme: web.sessions.lineage
coverage:
  primary:
    - web.sessions.lineage
    - web.sessions.permalink
  secondary:
    - session.spawn
    - session.lineage
risk: medium
live: true
provider: real-claude-code
preconditions:
  - UI-02 preconditions; `/api/agent/spawn` available (covered by ACP-08)
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/agents.$name.sessions.$id.tsx
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/session.$id.tsx:1-56
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/session-inspector.tsx:77-120
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/types.ts:11
  - /Users/pedronauck/Dev/compozy/agh/internal/session/spawn.go:14-200
steps:
  - Create parent session P via the UI (UI-02 flow).
  - From inside the parent agent transcript context, run a prompt that the agent will satisfy by spawning a child via the kernel CLI: "Spawn a worker child via `agh spawn --agent claude --ttl-seconds 600 --role worker -o json` and tell me its session id." (Or post `POST /api/agent/spawn` directly from the runner with the parent's identity headers and assert the SPA UI reflects it.)
  - Capture the child id C from the response.
  - Navigate to the parent's session permalink (or to the lineage view in the inspector). Assert the child appears in a lineage component (`SessionInspector` or sidebar — pick the surface the implementation actually exposes; if no lineage UI exists today, mark this scenario `unsupported_today` and link to the TechSpec follow-up rather than asserting a fictional control — truthful UI invariant).
  - Click the child entry (or use `runtime.url(`/session/${C}`)` permalink) → assert the SPA redirects via `session.$id.tsx:22-30` to `/agents/${agent_name}/sessions/${C}` and renders the child transcript.
  - Verify the child's events.db rows carry `parent_session_id = P` and `root_session_id = P`.
expected:
  - Permalink redirect resolves agent name correctly.
  - Child transcript route renders the chat shell, ChatHeader, and SessionInspector.
  - Lineage payload (`session.lineage.parent_session_id`) is exposed in the inspector or returned via UDS for assertion.
  - If a lineage tree component is not yet implemented in the SPA, the scenario must explicitly fail with `unsupported_today` and reference an open TechSpec — DO NOT silently assert against a non-existent control (truthful UI).
evidence:
  - ui-09-screenshots/{parent-chat,child-permalink-redirect,child-chat}.png
  - ui-09-events.json showing parent/root/spawn fields on child events
failure_signatures:
  - Permalink route renders a 404 for a session with a known agent — `session.$id.tsx:31-55` regression.
  - Child rendered without parent context anywhere in the UI — situation surface drift (cross-link to ACP-08, ACP-11).
cleanup:
  - stop child then parent; runtime.dispose().
```

### UI-10 — Tasks: queue with pending / leased / completed / failed; admin-cancel; lease TTL countdown

```yaml qa-scenario
id: ui-10-tasks-runs-queue
title: Tasks detail-runs panel shows pending / leased / completed / failed runs; admin can cancel a leased run; lease TTL is visible and updates live; raw claim_token is never inlined
theme: web.tasks
coverage:
  primary:
    - web.tasks.runs_panel
    - web.tasks.admin_cancel
  secondary:
    - autonomy.task_runs
    - security.claim_token_redaction
risk: high
live: false
provider: mock-acp
preconditions:
  - Seed tasks + task_runs in mixed states via `runtime.requestOperatorJSON("/api/tasks", ...)` and `/api/task-runs/...` per `internal/api/contract/tasks.go`. At least one leased run with `lease_until` in the future; one pending; one completed; one failed.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/tasks.tsx:1-50
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/tasks.$id.tsx:1-40
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/tasks/components/tasks-detail-runs-panel.tsx:1-200
steps:
  - await page.goto(runtime.url("/tasks"));
  - await expect(page.getByTestId("tasks-shell")).toBeVisible();
  - Open the seeded task → tasks-detail-runs-panel renders.
  - Assert one row per run; each row carries the seeded `data-testid={`tasks-detail-runs-item-${run.id}`}` and shows status (`pending`, `leased`, `completed`, `failed`).
  - For the leased run: assert a TTL chip (countdown to `lease_until`) updates over 5s — capture two values and assert the second is smaller.
  - Admin-cancel the leased run via the row action (or via the page-level cancel control if exposed). Confirm the run transitions to `cancelled` (or `failed` with `release_reason="admin_cancel"` per `internal/CLAUDE.md` autonomy spec) within ≤3s.
  - DOM scan: `match(/agh_claim_[A-Za-z0-9_-]+/g)` over the entire panel HTML returns null. The panel may show `claim_token_hash` (`sha256:...`) but never the raw token.
expected:
  - Panel reflects daemon truth via `useTasks` + `useTask` (`web/src/systems/tasks/hooks/`); pending/leased/completed/failed transitions are observed live.
  - Cancel action calls the documented API (`POST /api/task-runs/:id/cancel` or equivalent — pick whichever the implementation actually exposes; see `internal/api/spec/spec.go` `task-runs` group at `httpapi/routes.go:217-225`).
  - Lease TTL countdown does not freeze; updates at ≥1Hz (driven by a timer or a query refetch interval).
  - DOM has no raw `agh_claim_*` tokens (release blocker if it does).
evidence:
  - ui-10-screenshots/{queue-mixed,ttl-tick,after-cancel}.png
  - ui-10-network.har
  - ui-10-dom-needle-scan.json (claim_token regex count == 0)
failure_signatures:
  - TTL chip frozen — refetchInterval missing or component memoization too aggressive.
  - Admin-cancel returns success but the run stays `leased` — handler regression in `internal/autonomy`.
  - DOM contains a raw `agh_claim_` token — security release blocker.
cleanup:
  - revert seeded data; runtime.dispose().
```

### UI-11 — Truthful UI invariant: control NOT advertised by daemon does NOT render

```yaml qa-scenario
id: ui-11-truthful-ui-capability-flag
title: When the daemon does NOT advertise a capability (e.g. AGH Network disabled), the corresponding sidebar entry / settings panel / shortcut MUST NOT render
theme: web.truthful_ui
coverage:
  primary:
    - web.truthful_ui
  secondary:
    - api.parity.http
    - dx.cliff
risk: high
live: false
provider: mock-acp
preconditions:
  - Boot daemon with config that disables AGH Network (e.g. `[network] enabled = false` in the rendered config). Or seed a settings response in which a known capability flag is OFF.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app.tsx:30-60
  - /Users/pedronauck/Dev/compozy/agh/web/src/components/app-sidebar.tsx:96-119
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/network.tsx:35-70 (disabled state)
  - /Users/pedronauck/Dev/compozy/agh/internal/config/network.go (capability flag)
steps:
  - Visit `/`. Assert sidebar nav entries match the documented set when network is enabled.
  - Verify `agh status -o json` includes the flag set (`network.enabled = false`) by `runtime.requestOperatorJSON("/api/daemon/status")`.
  - Visit `/network` directly. Assert the route renders an explicit "Network is disabled" empty state (`network-disabled-state` test id from `web/src/routes/_app/network.tsx`), NOT a populated workspace.
  - Visit any settings panel that gates on a known capability (`/settings/network`); assert the disabled state surface OR the panel still renders for configuration (which one is the truthful behavior depends on the implementation — assert exactly the implemented surface).
  - **DX-cliff audit**: walk the entire static route tree (`web/src/routeTree.gen.ts`) — for each route, capture its rendered DOM and grep for any `<a>` / `<button>` whose target / handler points to an endpoint that does NOT exist in `web/src/generated/agh-openapi.d.ts`. Flag every orphan as a release blocker.
  - **Static audit (extra)**: run `grep -RIn "TODO\|placeholder\|FIXME" web/src/{routes,systems,components}/ --include='*.tsx'` and assert the count of UI-visible TODOs is 0.
expected:
  - When network is disabled: sidebar still allows navigating to `/network` (so deep links don't break), but the route shows the disabled empty state. There is NO control on the home dashboard, no shortcut anywhere else, that pretends network is on.
  - DX-cliff audit returns 0 orphan controls.
  - Static UI-visible TODO audit returns 0.
evidence:
  - ui-11-screenshots/{home-network-disabled,network-route-disabled,settings-network-route}.png
  - ui-11-orphan-controls.json (must be empty)
  - ui-11-static-todo-scan.txt
failure_signatures:
  - `/network` route renders a populated workspace despite the flag being off — capability gate regression.
  - Sidebar offers a disabled action that does nothing on click — DX-cliff (release blocker per truthful-UI invariant).
  - Orphan control points to an OperationID not present in `agh-openapi.d.ts` — codegen drift or static UI regression.
cleanup:
  - re-enable network in config; runtime.dispose().
```

### UI-12 — Accessibility: keyboard reach, focus rings, aria-live for SSE region, color contrast

```yaml qa-scenario
id: ui-12-accessibility
title: Every interactive control along the chat lifecycle is reachable by keyboard with a visible focus ring; the streaming region is an aria-live=polite landmark; color contrast meets DESIGN.md tokens; axe-core finds zero serious / critical violations
theme: web.accessibility
coverage:
  primary:
    - web.a11y.keyboard
    - web.a11y.aria_live
    - web.a11y.contrast
  secondary:
    - design.tokens
risk: high
live: true
provider: real-claude-code
preconditions:
  - UI-02 preconditions
  - axe-core injected via `@axe-core/playwright`
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/components/connection-indicator.tsx:38-52 (aria-live=polite)
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/session-resume-failure.tsx:34 (aria-live=assertive)
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/agent/components/agent-sessions-list.tsx:136 (aria-live=polite)
  - /Users/pedronauck/Dev/compozy/agh/DESIGN.md (color/contrast tokens)
steps:
  - Navigate the SPA from the home dashboard through onboarding (if not yet completed) → sessions list → new session → chat. Use only Tab / Shift+Tab / Enter / Space / Escape — no mouse.
  - At each focus stop, screenshot the focused element; assert a visible `outline` / `box-shadow:none` (per DESIGN.md flat-depth rule, focus uses `outline` instead of shadow, color `--color-accent`).
  - Inspect the chat-view region: it MUST be inside an element with `role="region"` AND `aria-live="polite"` (or assistant-ui's equivalent). The streaming region MUST NOT be `aria-live="assertive"` (that is reserved for resume-failure / errors).
  - Run axe-core on:
      - / (home),
      - /knowledge,
      - /tasks,
      - /agents/$name,
      - /agents/$name/sessions/$id,
      - /settings/general,
      - /settings/hooks-extensions.
    Filter to `severity in ["serious","critical"]` violations; assert count == 0.
  - Color contrast: for each surface, sample foreground/background pairs against `DESIGN.md` tokens (Inter body 13/15px on `--color-text-primary`/`--color-text-secondary` over `--color-canvas`/`--color-surface`). Assert WCAG-AA contrast ratios.
  - Reduced-motion: re-run UI-02 with `prefers-reduced-motion: reduce`; assert no transform-spin / loop animations remain (per CLAUDE.md design polish).
expected:
  - Tab order is logical (header → sidebar → main content → composer); no focus traps outside dialogs/sheets.
  - Connection indicator chip flips ARIA announcement when the daemon goes offline/online.
  - axe-core: 0 serious/critical violations across all listed routes.
  - Contrast ratios: ≥4.5 for body text, ≥3.0 for UI components and bold ≥18pt text.
evidence:
  - ui-12-axe.json (per route)
  - ui-12-screenshots/focus/*.png (per Tab stop)
  - ui-12-contrast.json (per pair)
failure_signatures:
  - Focus ring invisible on a control — DESIGN.md rule violation.
  - Streaming region is `aria-live="assertive"` — annoying screen-reader floods.
  - axe-core: ≥1 serious/critical violation — release blocker.
cleanup:
  - delete session; runtime.dispose().
```

### UI-13 — Responsive: 1440x900, 1024x768, 768x1024 (portrait), 390x844 (mobile)

```yaml qa-scenario
id: ui-13-responsive
title: Every primary route renders at four breakpoints with no horizontal overflow, no clipped content, working sidebar collapse / sheet patterns
theme: web.responsive
coverage:
  primary:
    - web.responsive.desktop
    - web.responsive.laptop
    - web.responsive.tablet
    - web.responsive.mobile
  secondary:
    - web.layout._app
risk: medium
live: false
provider: mock-acp
preconditions:
  - Workspace seeded with at least one session and one task (mock-acp fixtures)
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/_app.tsx
  - /Users/pedronauck/Dev/compozy/agh/web/src/components/app-sidebar.tsx
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/session-inspector.tsx (responsive sheet pattern via `Sheet` primitive)
steps:
  - For each viewport in [{w:1440,h:900}, {w:1024,h:768}, {w:768,h:1024}, {w:390,h:844}]:
      - page.setViewportSize({...});
      - page.goto runtime.url("/"), runtime.url("/agents/claude/sessions/$ID"), runtime.url("/knowledge"), runtime.url("/tasks"), runtime.url("/settings/general"), runtime.url("/network").
      - For each route: assert document.documentElement.scrollWidth <= viewport.width (no horizontal overflow); assert no element is clipped (`getBoundingClientRect` of every interactive control with text content fits inside viewport).
      - Screenshot each route per viewport.
  - On mobile (390x844): the SessionInspector should collapse into a Sheet (open via `PanelRightOpen` trigger, per `web/src/systems/session/components/session-inspector.tsx`); confirm.
  - On tablet portrait (768x1024): the sidebar should collapse to icon-rail mode; the workspace pills remain reachable.
expected:
  - No horizontal scrollbar at any of the four breakpoints on any of the listed routes.
  - Mobile chat composer remains visible and pinned to the bottom of the viewport with the keyboard hidden.
  - Sidebar collapse honors `useSidebarStore` (`web/src/stores/sidebar-store.ts`).
evidence:
  - ui-13-screenshots/{viewport}/{route}.png (16 screenshots minimum)
  - ui-13-overflow.json (max scrollWidth per page; should equal viewport.width)
failure_signatures:
  - Horizontal scrollbar on any viewport — layout regression.
  - Inspector pane permanently visible on mobile — Sheet not wired.
cleanup:
  - runtime.dispose().
```

### UI-14 — Theme adherence: flat depth, warm-dark palette, no shadows, signal palette correctness

```yaml qa-scenario
id: ui-14-design-tokens
title: Every primary route honors DESIGN.md — no `box-shadow` on content, no content gradients, accent/success/danger/warning/info map to documented hex tokens
theme: web.design
coverage:
  primary:
    - design.tokens
    - design.flat_depth
    - design.signal_palette
  secondary:
    - web.layout._app
risk: medium
live: false
provider: mock-acp
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/DESIGN.md
  - /Users/pedronauck/Dev/compozy/agh/packages/ui/src/tokens.css (--color-accent #E8572A, --color-canvas #141312, --color-canvas-deep #0E0E0F)
  - /Users/pedronauck/Dev/compozy/agh/web/src/styles.css
steps:
  - For each primary route (home, chat, knowledge, tasks, settings/general, settings/hooks-extensions, network, bridges, automation jobs, automation triggers):
      - Walk every element via `document.querySelectorAll('*')`; for each element: get `window.getComputedStyle(el)`. Reject the page if any element with non-zero text/icon content has `box-shadow !== "none"` AND is not inside a `[data-allow-shadow]` opt-in (the only documented exception is the marketing site sticky header — which does not apply to the SPA).
      - Reject the page if any element has `background-image` containing `linear-gradient(` (DESIGN.md "no gradients on content").
      - Sample `--color-accent` / `--color-canvas` / `--color-canvas-deep` / `--color-divider` / `--color-text-primary` from `:root`; assert exact hex values match `packages/ui/src/tokens.css`.
      - Walk every Pill / Chip / Pill.Dot; assert the rendered color resolves to one of the five documented signal hexes (`#E8572A`, `#30D158`, `#FF453A`, `#FFD60A`, `#BF5AF2`).
expected:
  - 0 elements with `box-shadow !== "none"` and visible content.
  - 0 content gradients.
  - All signal-tone chips map to the documented hex.
  - `:root` exposes the documented tokens.
evidence:
  - ui-14-violations.json (paths to any offending elements; should be empty)
  - ui-14-tokens.json (resolved values from :root)
failure_signatures:
  - Any element with `box-shadow: 0 1px 2px ...` — DESIGN.md flat-depth violation.
  - A Pill rendering with an undocumented hex — token override regression.
cleanup:
  - runtime.dispose().
```

### UI-15 — Tab visibility hidden 60s, then foreground: SSE catches up via `after_seq`; no duplicate messages

```yaml qa-scenario
id: ui-15-tab-visibility-reconnect
title: Hiding the browser tab for 60s while the daemon emits new events does not lose data; on visibility-change the SPA reconnects via after_seq; no duplicates
theme: web.chat.visibility
coverage:
  primary:
    - sse.replay
    - sse.last_event_id
  secondary:
    - web.chat.streaming
risk: medium
live: true
provider: real-claude-code
preconditions:
  - UI-02 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/hooks/use-session-chat-runtime.ts:34-52
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/session_stream.go:69-100
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/handlers.go:521
steps:
  - Create session and start a long prompt (≥60s of expected output).
  - After 5s of streaming, simulate tab hidden:
      `await page.evaluate(() => Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true }) && document.dispatchEvent(new Event('visibilitychange')));`
  - Wait 60s wall-clock. Daemon continues to produce tokens during this window; events are durably appended to events.db.
  - Foreground the tab:
      `await page.evaluate(() => Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true }) && document.dispatchEvent(new Event('visibilitychange')));`
  - Watch the chat-view: tokens that were emitted while hidden should now render (caught up via after_seq) without duplicating any earlier text.
  - At `finish`, compare DOM textContent vs `agh session transcript $S -o json`: must be byte-equal modulo whitespace.
  - Duplicate-window scan: same regex as UI-03 (no 80-char substring repeated with offset ≥ 200 chars).
expected:
  - Network HAR: a single SSE EventSource (or fetch-stream) reconnect happens at the visibility transition; the request includes `Last-Event-ID` derived from the last delta the SPA had.
  - DOM final text == events.db replay text.
  - 0 duplicate-window matches.
evidence:
  - ui-15-network.har
  - ui-15-dom-vs-events-diff.txt (must be empty)
  - ui-15-screenshots/{pre-hide,post-foreground,after-finish}.png
failure_signatures:
  - SSE source closes and never reconnects on foreground — visibility-change wiring missing in `useChatRuntime` (`web/src/systems/session/hooks/use-session-chat-runtime.ts`).
  - Duplicate text on foreground — Last-Event-ID not honored or set.
cleanup:
  - delete session; runtime.dispose().
```

### UI-16 — Error toasts: every typed error from BaseHandlers maps to a user-facing toast with action affordance

```yaml qa-scenario
id: ui-16-error-toasts
title: Triggering each typed error class (validation 400, conflict 409, not_found 404, forbidden 403, server 500) surfaces a toast with a plain-language message and an action affordance; raw error JSON is never shown
theme: web.errors
coverage:
  primary:
    - web.errors.toast
    - api.parity.error_envelope
  secondary:
    - dx.cliff
risk: medium
live: false
provider: mock-acp
preconditions:
  - daemon running; SPA reachable
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/lib/api-client.ts:24-65
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes/__root.tsx:23 (Toaster)
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/permission-prompt.tsx:30
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/session/hooks/use-session-create-dialog.ts:185
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/workspace/hooks/use-workspace-setup-content.ts:71
steps:
  - For each error class:
      - Mock the relevant HTTP route via `page.route(...)` to return the typed error envelope (status code + `{error: "<reason>", code: "<stable_code>"}` per `internal/api/core/error_paths_test.go`).
      - Trigger the action that uses that route (create session with conflict id, approve permission with stale request id, save settings with unrelated workspace id, etc.).
      - Capture the toast content via `page.locator("[data-sonner-toast]")` and read its inner text.
      - Assert the toast text is plain language (no JSON braces, no stack trace), references the user's action ("Couldn't create session: that name is already in use"), and includes a retry/dismiss affordance.
expected:
  - Each error class produces exactly one toast.
  - Toast text never contains JSON syntax or HTTP status codes.
  - The "View details" affordance, where present, opens a side-sheet with the raw envelope for diagnostics — gated behind an explicit click.
  - 5xx errors flip the connection indicator (`web/src/components/connection-indicator.tsx`) into a `reconnecting`/`disconnected` chip when appropriate.
evidence:
  - ui-16-screenshots/{error-class}.png (5 screenshots)
  - ui-16-toasts.json (per-class final text)
failure_signatures:
  - Toast inner text contains `{` or `"error":` — error mapper regression.
  - Multiple toasts per single failure — duplicate-fire from React 19 strict-mode regression.
  - 4xx error flips the connection indicator to disconnected — false-positive disconnect.
cleanup:
  - clear all toasts; runtime.dispose().
```

### UI-17 — claim_token redaction across the SPA

```yaml qa-scenario
id: ui-17-claim-token-redaction
title: Across every primary route, deep DOM text scrape and HAR body inspection finds zero `agh_claim_*` raw tokens; `claim_token_hash` (`sha256:...`) is the only acceptable surface
theme: web.security
coverage:
  primary:
    - security.claim_token_redaction
  secondary:
    - web.tasks.runs_panel
    - web.chat.tool_calls
risk: high
live: true
provider: real-claude-code
preconditions:
  - Seed (a) a real Claude Code session that triggers a synthetic prompt with `PromptSyntheticMeta` carrying a fake claim_token (per ACP-18 fixture) AND (b) one task_run row with a known `claim_token_hash`.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md (Security Invariants)
  - /Users/pedronauck/Dev/compozy/agh/internal/acp/types.go:175-184
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems/tasks/components/tasks-detail-runs-panel.tsx
steps:
  - Visit each primary route after triggering the seeded synthetic prompt:
      - /, /agents/claude/sessions/$ID (during streaming, after finish), /knowledge, /tasks, /tasks/$ID, /settings/{general, hooks-extensions, vault}, /network.
  - For each route: capture full document.body.innerHTML AND innerText AND every Network HAR response body.
  - Run regex `/agh_claim_[A-Za-z0-9_-]+/g` over each artifact; assert null match.
  - Allow `sha256:...` patterns where the hash field is the only legitimate exposure; assert these never appear in user-facing labels (only in a hidden inspector pane gated behind a click).
expected:
  - 0 raw tokens across all artifacts.
  - Hash strings appear only in inspector-style hidden panes; never in toasts, page titles, or sidebar labels.
evidence:
  - ui-17-dom-needles.json (counts per file; all 0)
  - ui-17-har-needles.json (counts per body; all 0)
failure_signatures:
  - Any non-zero count → release blocker (CLAUDE.md "claim_token redaction is non-negotiable"). Cross-link to ACP-18.
cleanup:
  - delete seeded data; runtime.dispose().
```

### UI-18 — COPY.md vocabulary scrape: no `recipe`, `workflow`, `procedure`, `playbook` for AGH artifacts; canonical `capability` enforced

```yaml qa-scenario
id: ui-18-copy-vocabulary
title: Every visible UI string passes the COPY.md canonical-term gate; AGH artifacts are called `capability`; backend nouns match the runtime vocabulary
theme: web.copy
coverage:
  primary:
    - copy.canonical_terms
  secondary:
    - dx.cliff
risk: medium
live: false
provider: mock-acp
preconditions:
  - SPA serving from runtime.url("/"); workspace seeded
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/COPY.md
  - /Users/pedronauck/Dev/compozy/agh/docs/_memory/glossary.md
steps:
  - Walk every primary route (same list as UI-13) and capture document.body.innerText.
  - Assemble a single text blob; run forbidden-word regex (case-insensitive, word-boundary-anchored): `/\b(recipe|workflow|procedure|playbook)\b/g`.
  - Assert: count of forbidden matches == 0 OR the only matches are in user-authored content (e.g. a memory body the operator typed) — assert by tagging UI-rendered chrome via `data-copy="ui"` or by allowlist of safe contexts (composer transcripts, displayed memory body content). Where the forbidden word is part of a legitimate quote (e.g. an upstream library name), record the exception in `ui-18-allowlist.json` with rationale.
  - Assert the canonical term `capability` appears in the rendered chrome at least once on the skills/capability surfaces (`/skills`, `/agents/$name`).
  - For internal-only audit: `grep -RIn '\b(recipe|workflow|procedure|playbook)\b' web/src/{routes,systems,components}/ --include='*.tsx' --include='*.ts'` — count must be 0 outside test fixtures and storybook stories that explicitly demonstrate forbidden terms (the storybook story documenting acceptable v inacceptable language is itself the only allowed home for such literals).
expected:
  - 0 forbidden matches in rendered chrome.
  - Canonical term `capability` reachable via the skills/agent surfaces.
  - Internal scan: 0 forbidden matches outside the explicit allowlist.
evidence:
  - ui-18-dom-strings.json (full text blob per route)
  - ui-18-violations.json (per match: route, element selector, surrounding context)
  - ui-18-allowlist.json (any approved exceptions with rationale)
failure_signatures:
  - Any UI chrome string contains `recipe`/`workflow`/`procedure`/`playbook` outside the allowlist — COPY.md / glossary violation.
cleanup:
  - runtime.dispose().
```

### UI-19 — DX-cliff: no fake auth method, no orphan controls, no UI-only TODOs

```yaml qa-scenario
id: ui-19-dx-cliff
title: Static + runtime audit refuses any control whose backing operation is missing, any settings panel that pretends to expose an auth/feature method the daemon doesn't implement, and any UI-visible TODO/FIXME marker
theme: web.dx_cliff
coverage:
  primary:
    - dx.cliff
    - web.truthful_ui
  secondary:
    - copy.canonical_terms
risk: high
live: false
provider: mock-acp
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/src/generated/agh-openapi.d.ts
  - /Users/pedronauck/Dev/compozy/agh/web/src/routes
  - /Users/pedronauck/Dev/compozy/agh/web/src/systems
steps:
  - **Static audit**:
      - For each `useMutation`/`useQuery` call across `web/src/`, extract the API path (literal or template-built). Cross-reference with the operation list in `web/src/generated/agh-openapi.d.ts`. Any path that does not resolve to an operation is a release blocker (codegen drift).
      - Run `grep -RIn 'TODO\|FIXME\|XXX' web/src/{routes,systems,components}/ --include='*.tsx' --include='*.ts' | grep -v '\.test\.\|stories'`; assert count == 0 for human-visible files (excluding tests and stories).
      - Run `grep -RIn 'fake\|mock-only\|temporary' web/src/{routes,systems,components}/ --include='*.tsx' | grep -v '\.test\.\|stories\|mocks/'`; assert count == 0.
  - **Runtime audit**:
      - For every settings panel (`/settings/{general,providers,mcp-servers,memory,skills,automation,network,observability,hooks-extensions,vault}`), assert each form field maps to a documented field in the corresponding `OperationRequest<...>` type from the OpenAPI spec.
      - For each form field whose backing API does not implement the underlying behavior in the current daemon build (e.g. an `auth_method: "oauth"` selector when the daemon only supports `api_key`): the field MUST NOT render. Inspect the rendered DOM, list every `<select>` / `<input>` and assert presence/absence against the implemented capability matrix.
expected:
  - 0 orphan API paths.
  - 0 UI-visible TODO/FIXME markers.
  - 0 unimplemented form fields rendered.
evidence:
  - ui-19-orphan-paths.json
  - ui-19-todos.json
  - ui-19-form-field-audit.json (per panel: declared vs rendered)
failure_signatures:
  - Any orphan API path → codegen drift or aspirational UI (release blocker).
  - Any UI-visible TODO/FIXME → DX cliff.
  - Any rendered "auth_method=oauth" without implementation → truthful UI violation.
cleanup:
  - runtime.dispose().
```

### UI-20 — Real-LLM regression spec deterministic by structure, not text

```yaml qa-scenario
id: ui-20-real-llm-deterministic
title: A Playwright spec runs end-to-end against real Claude Code in the test-e2e-web lane; assertions are over event-sequence shape and DOM structure, never over token text content
theme: web.qa.regression
coverage:
  primary:
    - web.chat.streaming
    - sse.typed_envelope
    - web.qa.deterministic
risk: high
live: conditional
provider: real-claude-code
preconditions:
  - UI-02 preconditions
  - `make test-e2e-web` lane reachable (Playwright + daemon binary built); `AGH_TEST_DAEMON_BIN` set
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/web/playwright.config.ts:9-29
  - /Users/pedronauck/Dev/compozy/agh/Makefile:29-30 (test-e2e-web target)
  - /Users/pedronauck/Dev/compozy/agh/web/e2e/fixtures/runtime.ts:1-615
steps:
  - Run `make test-e2e-web` with the lab's bootstrap.env exported; the live spec MUST be skipped automatically when `ANTHROPIC_API_KEY` is absent (use `test.skip(!process.env.ANTHROPIC_API_KEY, "...")` at the top of the spec).
  - Inside the spec: create a session, send a single short prompt ("Read README.md and reply with one sentence"), wait for `finish`.
  - Assertions (deterministic by design):
      - DOM contains exactly one user message (`getByRole("group", { name: /user/i })` count == 1) — assert structure, not text.
      - DOM contains at least one assistant message; assistant message is non-empty (length > 0) — DO NOT assert specific text content.
      - SSE network frames contain at least one `text-delta`, exactly one `text-start`, exactly one `text-end`, exactly one `finish`.
      - events.db contains at least one `agent_message` row with non-empty content.
      - The composer is re-enabled after `finish` (`composer-send-button` is clickable).
  - When `ANTHROPIC_API_KEY` is absent the lane runs in `live: conditional` mode and skips this scenario — record the skip in the summary as `Blocked: provider key absent`.
expected:
  - Lane runs green when the key is present; cleanly skipped (Blocked) when absent.
  - No flake driven by token text variability across runs (verified by 5 reruns: pass/pass/pass/pass/pass).
evidence:
  - ui-20-runs-summary.json (5 reruns; pass count 5/5 expected)
  - ui-20-network.har
  - ui-20-events.json
failure_signatures:
  - Spec asserts `expect(text).toBe("The repository contains ...")` — flake-prone (structural-only is mandatory).
  - Spec passes when key absent — skip gate missing.
cleanup:
  - delete sessions; runtime.dispose().
```

## 8. Edge cases

- **Empty daemon (no workspaces yet)**: `_app.tsx:30-31` short-circuits to `WorkspaceOnboarding`. UI-01 covers loading; an explicit edge case here: refusing to navigate to `/agents/$name/sessions/$id` while onboarding is unresolved must redirect to onboarding (pull from `useAppLayout`).
- **CORS**: `httpapi/middleware.go` exposes `Last-Event-ID` only via the configured CORS allowlist; the SPA running on the daemon's same origin does not exercise this, but the QA runner must verify when running Vite dev (`make web-dev`) that the proxy preserves the header.
- **Sonner queue overflow**: more than 5 simultaneous toasts collapse into a stack; UI-16 must not regress this UX.
- **Slow network (cellular profile)**: visibility-change reconnect (UI-15) under a 4G profile (Playwright `context.setOffline(true)`/`context.setOffline(false)`) — backpressure in the SPA.
- **Browser back/forward across `/settings/*`**: covered by `settings.spec.ts`; real-LLM lane re-tests after restart-required save (UI-07).
- **Composer clear during streaming**: `composer-clear-dialog` (`session-thread.tsx:213`); explicit confirm/cancel flow must not race with the streaming delta state.
- **PermissionPrompt with timeout**: `defaultPermissionWait = 5*time.Minute` (per `internal/acp/client.go:27`); the SPA must surface a deadline indicator on the prompt card (extract from the implementation; if absent, mark as TechSpec follow-up rather than asserting a fictional control).
- **Concurrent two prompts on same session**: covered by ACP-15 on the daemon side; the SPA composer should be disabled while a prompt is in flight (`canPrompt` from `useSessionPageControls`).
- **Long workspace path**: workspace pill should truncate with `title` attr for hover; visual check at UI-13 mobile size.
- **`X-AGH-Workspace-ID` mismatch via deep link**: when the operator navigates to a session id that belongs to another workspace, the `_app/agents.$name.sessions.$id.tsx:107-112` toast fires "Session not found" and redirects to `/agents/$name`. Re-test after authenticating against a workspace switcher.

## 9. Integration surfaces

| Surface | Kind | File:line refs |
|---|---|---|
| `GET /api/daemon/status` | JSON | `web/src/systems/daemon/adapters/daemon-api.ts:18-30`; consumed by `useHomePage` (`web/src/hooks/routes/use-home-page.ts`). |
| `GET /api/observe/health` | JSON | `web/src/systems/daemon/adapters/daemon-api.ts` (`fetchHealth`). |
| `POST /api/sessions/:id/prompt` (SSE) | text/event-stream | `web/src/systems/session/hooks/use-session-chat-runtime.ts:26-32` via `AssistantChatTransport`. |
| `POST /api/sessions/:id/prompt/cancel` | JSON | reachable from chat header Stop control (cross-link to `internal/api/httpapi/sessions.go:43-50`). |
| `GET /api/sessions/:id/transcript` | JSON | `web/src/systems/session/lib/query-options.ts:sessionTranscriptOptions`. |
| `POST /api/sessions/:id/approve` | JSON | `web/src/systems/session/components/permission-prompt.tsx:24-35`. |
| `GET /api/agent/context` | JSON | `web/src/systems/agent` for the situation surface (cross-link to ACP-11). |
| `POST /api/agent/spawn` | JSON | child-session creation surface (cross-link to UI-09). |
| `GET/PUT/PATCH /api/settings/*` | JSON | `web/src/systems/settings/adapters/`. |
| `POST /api/memory/consolidate` | JSON | `web/src/systems/knowledge/adapters/knowledge-api.ts` (`consolidateMemory`). |
| `POST /api/bridges/:id/test-delivery` | JSON | `web/src/systems/bridges/adapters/`. |
| `Last-Event-ID` SSE replay | header | `web/src/systems/session/hooks/use-session-chat-runtime.ts` reconnect path; `internal/api/core/handlers.go:521`. |
| Vite proxy override | env | `web/src/lib/vite-api-proxy-target.ts:1-22`. |

## 10. Failure modes

| Mode | Surface | Detection |
|---|---|---|
| Token rendering frozen / non-incremental | Chat | UI-02 monotonic-growth assertion |
| Detached prompt cancelled by tab close | Chat | UI-03 reload mid-stream |
| Cancel slow (>2s) | Chat | UI-04 wall-clock assertion |
| Tool input/output collapsed wrong / leaks tokens | Chat | UI-05 collapsibility + UI-17 redaction scan |
| Hot-apply false-positive restart banner | Settings | UI-07 subtest A |
| Restart banner lies (PID unchanged) | Settings | UI-07 subtest B |
| Disabled capability still rendered | Truthful UI | UI-11 + UI-19 |
| Keyboard trap inside chat | Accessibility | UI-12 |
| `aria-live` floods on assertive level | Accessibility | UI-12 |
| Horizontal overflow at any breakpoint | Responsive | UI-13 |
| `box-shadow` or content gradient | Design | UI-14 |
| Reconnect after visibility loses tokens | Chat | UI-15 |
| Toast text contains JSON | Errors | UI-16 |
| Raw `agh_claim_*` in DOM/HAR | Security | UI-17 |
| `recipe` / `workflow` / `playbook` in chrome | Copy | UI-18 |
| Orphan control / TODO / unimplemented form field | DX | UI-19 |
| Real-LLM spec flakes on text content | Regression | UI-20 |

## 11. Fixtures

- **Bootstrap manifest**: produced by `agh-qa-bootstrap`; includes unique `AGH_HOME`, daemon ports, `PROVIDER_HOME`/`PROVIDER_CODEX_HOME`, **`AGH_WEB_API_PROXY_TARGET`** (mandatory).
- **Workspace seed**: `$LAB/workspace/{README.md, src/file_a.go, src/file_b.go, generated_long_file.txt(~2MB)}` — same as `03-acp-sessions.md` ACP-16. Required for UI-02, UI-05, UI-09, UI-15, UI-20.
- **Memory seed (UI-06)**: 3+ memories of mixed types/scopes via `runtime.requestOperatorJSON` POST; deleted in cleanup.
- **Bridges seed (UI-08)**: 1 bridge with a known adapter and a malformed payload sample.
- **Tasks/runs seed (UI-10)**: tasks across all four states (`pending`, `leased`, `completed`, `failed`) with documented `claim_token_hash` (NEVER raw token) and varying `lease_until` distances.
- **Capability disabled config (UI-11)**: rendered config with `[network] enabled = false`.
- **claim_token fake (UI-17)**: synthetic prompt fixture with `agh_claim_FAKE_QA_*` token; per ACP-18.
- **Provider auth**: direct `claude` uses native Claude CLI auth from the
  effective Claude home for the lane (operator `HOME` by default; isolated
  `PROVIDER_HOME` only for explicit isolated-home scenarios). Bound-secret
  lanes stage their credentials into `PROVIDER_HOME`.
- **Forbidden needles**: `["agh_claim_FAKE_QA_", "agh_claim_TESTONLY_"]` for UI-17; runner sweeps DOM and HAR.
- **axe-core (UI-12)**: `@axe-core/playwright`; rules pinned to WCAG 2.1 AA; serious/critical filter only.
- **Mock fixtures (UI-01, UI-06, UI-07, UI-08, UI-10, UI-11, UI-13, UI-14, UI-16, UI-18, UI-19)**: `internal/testutil/acpmock/testdata/*.json` plugged via `runtimeOptions.seed.mockAgents[]` (`web/e2e/fixtures/runtime.ts:21-32`).
- **Reduced-motion profile (UI-12)**: Playwright `emulateMedia({ reducedMotion: "reduce" })`.

## 12. Citations

- Repo-wide rules: `/Users/pedronauck/Dev/compozy/agh/CLAUDE.md` (Critical Rules; Workflow; Skill Dispatch; Design System; Copy System; CI/Release).
- Backend invariants: `/Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md` — Architecture, Concurrency, Observability, Security Invariants.
- Web rules: `/Users/pedronauck/Dev/compozy/agh/web/CLAUDE.md` (Design System, Copy System, Skill Dispatch, Frontend Architecture Rules).
- Design tokens: `/Users/pedronauck/Dev/compozy/agh/DESIGN.md` and `/Users/pedronauck/Dev/compozy/agh/packages/ui/src/tokens.css`.
- Copy system: `/Users/pedronauck/Dev/compozy/agh/COPY.md` and `/Users/pedronauck/Dev/compozy/agh/docs/_memory/glossary.md`.
- Web SPA structure: `/Users/pedronauck/Dev/compozy/agh/web/src/{routes,systems,components,hooks,integrations,stores,lib}` (per `web/CLAUDE.md` Structure).
- Embedded SPA: `/Users/pedronauck/Dev/compozy/agh/web/embed.go`.
- Vite proxy: `/Users/pedronauck/Dev/compozy/agh/web/src/lib/vite-api-proxy-target.ts`.
- Chat runtime:
  - `/Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/agents.$name.sessions.$id.tsx:1-152`.
  - `/Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/session-chat-runtime-provider.tsx:1-46`.
  - `/Users/pedronauck/Dev/compozy/agh/web/src/systems/session/hooks/use-session-chat-runtime.ts:1-52`.
  - `/Users/pedronauck/Dev/compozy/agh/web/src/components/assistant-ui/session-thread.tsx:1-300+`.
- Tool rendering: `/Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/tool-call-card.tsx`, `/Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/tool-renderers/`.
- Permission flow: `/Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/permission-prompt.tsx:1-60+`.
- Inspector: `/Users/pedronauck/Dev/compozy/agh/web/src/systems/session/components/session-inspector.tsx`.
- Knowledge: `/Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/knowledge.tsx`, `/Users/pedronauck/Dev/compozy/agh/web/src/systems/knowledge/`.
- Settings: `/Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/settings/*.tsx`, `/Users/pedronauck/Dev/compozy/agh/web/src/systems/settings/components/settings-restart-banner.tsx:23-100`.
- Bridges: `/Users/pedronauck/Dev/compozy/agh/web/src/systems/bridges/components/`.
- Tasks: `/Users/pedronauck/Dev/compozy/agh/web/src/systems/tasks/components/tasks-detail-runs-panel.tsx`, `/Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/tasks*.tsx`.
- Network: `/Users/pedronauck/Dev/compozy/agh/web/src/systems/network/components/network-workspace-shell.tsx`, `/Users/pedronauck/Dev/compozy/agh/web/src/routes/_app/network.tsx`.
- API client: `/Users/pedronauck/Dev/compozy/agh/web/src/lib/api-client.ts:1-65`, `/Users/pedronauck/Dev/compozy/agh/web/src/lib/api-contract.ts`.
- Generated contract: `/Users/pedronauck/Dev/compozy/agh/web/src/generated/agh-openapi.d.ts`.
- Connection indicator (`aria-live`): `/Users/pedronauck/Dev/compozy/agh/web/src/components/connection-indicator.tsx:23-52`.
- Playwright runtime: `/Users/pedronauck/Dev/compozy/agh/web/e2e/fixtures/runtime.ts:1-615`, `/Users/pedronauck/Dev/compozy/agh/web/e2e/fixtures/test.ts:1-50`, `/Users/pedronauck/Dev/compozy/agh/web/e2e/fixtures/selectors.ts`.
- Existing real-shape e2e specs (mock-driven baseline to avoid duplicating): `/Users/pedronauck/Dev/compozy/agh/web/e2e/{harness-smoke,session-onboarding,session-provider-override,settings,settings-transport,network,automation,bridges,tasks,tasks-coordinator-handoff,combined-flows,storybook-bootstrap}.spec.ts`.
- Make targets: `/Users/pedronauck/Dev/compozy/agh/Makefile:29-30 (test-e2e-web)`, `:81-98 (web-dev/web-build/web-test)`.
- ACP backend cross-links:
  - Detached prompt: `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/prompt.go:104`.
  - Typed envelope state machine: `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/prompt.go:251-580`.
  - Prompt cancel: `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/sessions.go:43-50`, `/Users/pedronauck/Dev/compozy/agh/internal/acp/client.go:594-610`.
  - SSE poll/replay: `/Users/pedronauck/Dev/compozy/agh/internal/api/core/session_stream.go:69-100`, `/Users/pedronauck/Dev/compozy/agh/internal/api/core/handlers.go:521`.
- QA framework references: `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md` (scenario shape, evidence-as-pass-criterion, transport-snapshot vs persisted-events parity); `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md` (hermetic env shield, async/cancel rigor, ≤2s cancel assertions).
