# AGH Home Dashboard — Implementation TechSpec

**Status:** READY FOR IMPLEMENTATION · 2026-07-23
**Design source of truth:** `docs/design/opendesign/dashboard/dashboard.html` (approved prototype, edit — never regenerate)
**Target surface:** home route `/` → OS window `dashboard` (`web/src/systems/os/apps/dashboard/`)
**Supersedes:** the current minimal `DashboardWindow` (daemon status card + 4 lifetime metrics)

This spec covers everything required — backend aggregates, API/contract/codegen co-ship, CLI parity, frontend data layer, component mapping to `@agh/ui`, states, interactions, and phased tasks — to implement the redesigned end-user home dashboard with maximum fidelity to the prototype.

---

## 1. Goals & non-goals

**Goals**

1. Replace the operator-scalar home with the 7-zone end-user dashboard: *what agents did, what needs you, what it costs*.
2. **Truthful UI**: every widget maps to persisted daemon data. Metrics that need new aggregation get real backend work in this spec — nothing is invented to fill layout (PRODUCT anti-slop directive; verified rule "Dashboard must be truthful UI").
3. **Agent-manageability**: every new aggregate ships on HTTP **and** UDS, plus a CLI verb with structured output (`agh observe overview -o json` — the prototype's System zone advertises exactly this).
4. Reuse `@agh/ui` primitives; promote genuinely generic pieces into `packages/ui` instead of forking per-domain copies.

**Non-goals**

- No redesign of the Tasks window dashboard (`TasksDashboardView`) beyond adopting the promoted panel primitive.
- No new charting/animation dependencies (see §8).
- No desktop widgets, no minimize tray (OS v2 contract).

---

## 2. Current state (evidence)

| Layer | Today | Reference |
|---|---|---|
| Route `/` | `createOsRouteSync("dashboard")` mounts OS shell; body is `DashboardWindow` | `web/src/routes/_app/index.tsx:12`, `web/src/systems/os/lib/app-registry.tsx:154` |
| Dashboard body | Daemon `StatusCard` + 4 `Metric`s (active sessions, workspaces, agents, uptime) — no charts, no approvals, no live runs | `web/src/systems/os/apps/dashboard/dashboard-window.tsx:68-160` |
| Rich pattern | `TasksDashboardView`: KPI strip → queue health (recharts) → status breakdown → active runs | `web/src/systems/tasks/components/tasks-dashboard-view.tsx:77-117` |
| Backend | Point-in-time snapshots only. **No day/hour bucketed aggregate exists anywhere** (only `GROUP BY` in store: session_input mode/status, loop_runtime by loop) | `internal/observe/tasks_dashboard.go:44`, `internal/store/globaldb/queries/session_input.sql:72` |

---

## 3. Design contract (locked — do not re-litigate)

Extracted from the prototype + standing verified rules. These are acceptance criteria, not suggestions.

1. **Zone order** (prototype `<main>` order, final):
   `pagemeta` → **Needs you** → **KPI strip** → row(**Working now** | **Network**) → **Pulse** (full-width) → row(**Outcomes** | **Usage & cost**) → row(**Agents** | **Activity**) → **System**.
2. **Head**: identity lives in the OS window head/topbar (glyph + "Home" + Live `ConnectionIndicator` + primary **New session**). The body renders **no H1** and no second identity (pagehead-redesign contract). `pagemeta` is a demoted 12px meta line (date · workspace).
3. **Accent budget**: accent appears only as the pulsing running-dot, the primary action, and chart-status where color *is* the data. **Never as a card/panel border** (verified rule "Dashboard live cards never use accent borders" — runcards use `--line`, hover `--line-strong`).
4. **Viz neutrality**: magnitude charts (usage line, sparkline, share bar, pulse heatmap, runtime bars) paint with `--color-viz-*` neutral ink only. Status colors (`success`/`danger`) are allowed **only** in the Outcomes stacked bars and status dots, where the series itself is that state (tokens.css `--color-viz-*` comment).
5. **Paired panels close flush**: every `grid-2` row bottom-aligns; surplus height is absorbed by a legitimate flexing element (chart `flex:1`, distributed rows), footers pin with `margin-top:auto`. Never empty air.
6. **Run controls map to live status**: Approve only on awaiting-approval, Retry only on failed-with-retryable-run, no invented controls.
7. Flat depth model: panels are `bg-canvas-soft` + hairline borders + `radius-lg`; shadows only on overlays/tooltips.
8. Reduced motion: count-up, pulse dots and tickers degrade per `prefers-reduced-motion`.
9. Persisted UI state: usage window (`7|30|90`) and System fold persist (localStorage), same keys semantics as prototype (`agh-home-win`, `agh-home-sys`).

---

## 4. Data truthfulness matrix

Every widget → source. **NEW** marks backend work this spec adds (§5). Fields cited from `internal/api/contract`.

| # | Widget (prototype `data-od-id`) | Data | Source | Status |
|---|---|---|---|---|
| 1 | `section-needs-you` rows + count | approvals / needs-input / failures with actions | `GET /api/observe/tasks/inbox` lanes `approvals`, `failed_runs`, `blocked` (`contract/task_catalog.go:254-315`) + overview `attention` block | **Partial → NEW** (unified attention list, server-side) |
| 2 | `kpi-working-now` | active sessions + active task runs | `/api/status` `sessions.active` (`contract/status.go:129`) + `/api/observe/tasks/dashboard` `active_runs` (`contract/tasks_dashboard.go:128`) | Exists |
| 3 | `kpi-needs-you` | attention count by type | overview `attention` totals | **NEW** |
| 4 | `kpi-completed-today` | runs completed today · tasks closed today | day-windowed run/task outcomes | **NEW** |
| 5 | `kpi-usage` + spark | 30d tokens, est. cost, daily series | workspace-wide daily usage rollup | **NEW** |
| 6 | `section-working-now` runcards | agent, current activity line, elapsed, kind | active user sessions + `SessionPayload.activity` (`contract/session_runtime_payloads.go:79`: `current_tool`, `turn_started_at`, iterations) + dashboard `active_runs.items[]` (task runs: `age_ms`, `task_title`, `run_status`) | Exists |
| 7 | `section-network` panel | peers online (+names), open work, channels, last activity | `GET /api/network/status` (`contract/network_payloads.go:11`) + `/api/workspaces/:id/network/peers` | Exists |
| 8 | `network-budget` meter | wakes used / limit, reset | `GET /api/workspaces/:id/network/usage` `budget` (`contract/network_coordination.go:106`) | Exists (limit from config) |
| 9 | "Messages today" | daily message count | `NetworkStatusPayload.messages_*` are **lifetime** counters | **NEW** (overview computes today window) — fallback: relabel "Messages" |
| 10 | `section-pulse` heatmap | hour×weekday event buckets, 14d | none today | **NEW** |
| 11 | `pulse-insights` | busiest hour · longest session · top tool | none today | **NEW** (derived in overview) |
| 12 | `section-outcomes` | per-day run outcomes 14d + success % | none today (dashboard totals are lifetime) | **NEW** |
| 13 | `section-usage` chart + window | per-day tokens for 7/30/90d (retention-bounded) + est. cost | none today (token stats are per-session, unbucketed — `internal/store/token_stats_cost.go:30`) | **NEW** |
| 14 | per-agent share bar | token share by agent, 30d | none today | **NEW** |
| 15 | `section-agents` rows | active, sessions, failed, runtime bar (7d) | `GET /api/agents/catalog` `AgentSessionMetricsPayload` (`contract/agent_catalog.go:6`: `total`, `active`, `failed`, `runtime_seconds`, `last_activity_at`) | **Partial → NEW** (add 7d windowing; today metrics are lifetime) |
| 16 | `section-activity` feed | recent events, tones, "Earlier today", quiet-events fold | `GET /api/logs` + `GET /api/logs/stream` (`contract/agent_observe_payloads.go:109`; parser `core/parsers.go:44`) — client buckets recency & quiet classes | Exists |
| 17 | `section-system` bar + tiles | version, uptime, providers, scheduler, memory, hooks today, retention | `/api/status`: `daemon.version/started_at`, `providers[]`, `automation`, `memory` (`contract/status.go:19-35`); hooks-today count via overview; retention from config | **Partial → NEW** (hooks-today; storage tile optional) |

**Copy/data reconciliation** (prototype copy vs. backend truth — resolve during implementation, daemon truth wins):

- "Messages today" → ship the windowed counter in overview (#9); if cut, relabel to "Messages".
- Agents "runtime · last 7 days" → requires windowed metrics (#15); if deferred, relabel to "runtime · all time".
- System "Retention 60 days" → read the observability retention config key; never hardcode.
- Attention "Retry" button → only if a retry/run-again verb exists for failed runs; otherwise ship "Open" only (rule #6, §3).

---

## 5. Backend work

### 5.1 New endpoint: `GET /api/observe/overview`

One workspace-scoped read model powering every **NEW** row above. Single endpoint (not five) because: the CLI advertises one verb (`agh observe overview`), the zones refresh together, and all aggregates share the same windowing/retention rules.

Query params: `workspace` (optional; empty = global/home scope, mirroring `taskScopeForActiveWorkspace` semantics), `usage_window` (`7|30|90`, default 30).

**Response DTO** — new file `internal/api/contract/observe_overview.go`:

```go
type ObserveOverviewResponse struct { Overview ObserveOverviewPayload `json:"overview"` }

type ObserveOverviewPayload struct {
    SchemaVersion string                   `json:"schema_version"`
    GeneratedAt   string                   `json:"generated_at"`
    Attention     OverviewAttentionPayload `json:"attention"`
    Today         OverviewTodayPayload     `json:"today"`
    Outcomes      OverviewOutcomesPayload  `json:"outcomes"`       // 14d fixed
    Usage         OverviewUsagePayload     `json:"usage"`          // usage_window, retention-bounded
    Pulse         OverviewPulsePayload     `json:"pulse"`          // 14d fixed
    Network       OverviewNetworkPayload   `json:"network"`        // today-windowed counters only
    System        OverviewSystemPayload    `json:"system"`         // hooks_today, retention_days
    Freshness     TaskDashboardFreshnessPayload `json:"freshness"` // reuse existing shape (contract/tasks_dashboard.go:162)
}

type OverviewAttentionPayload struct {
    Total int                      `json:"total"`
    ByKind map[string]int          `json:"by_kind"` // approval | needs_input | failure
    Items []OverviewAttentionItem  `json:"items"`
}
type OverviewAttentionItem struct {
    Kind       string `json:"kind"`                  // approval | needs_input | failure
    Title      string `json:"title"`                 // task/session title
    Detail     string `json:"detail,omitempty"`      // e.g. "provider timed out"
    TaskID     string `json:"task_id,omitempty"`
    RunID      string `json:"run_id,omitempty"`
    SessionID  string `json:"session_id,omitempty"`
    AgentName  string `json:"agent_name,omitempty"`
    OccurredAt string `json:"occurred_at"`
    Actions    []string `json:"actions"`             // approve | reject | open | reply | retry — only verbs the daemon accepts for this item
}

type OverviewTodayPayload struct {
    RunsCompleted int `json:"runs_completed"`
    RunsFailed    int `json:"runs_failed"`
    TasksClosed   int `json:"tasks_closed"`
}

type OverviewOutcomesPayload struct {
    Days       []OverviewOutcomeDay `json:"days"`     // ascending, 14 entries
    Completed  int                  `json:"completed"`
    Failed     int                  `json:"failed"`
    Canceled   int                  `json:"canceled"`
    SuccessPct float64              `json:"success_pct"`
}
type OverviewOutcomeDay struct {
    Date string `json:"date"` Completed int `json:"completed"` Failed int `json:"failed"` Canceled int `json:"canceled"`
}

type OverviewUsagePayload struct {
    WindowDays    int                    `json:"window_days"`
    RetentionDays int                    `json:"retention_days"`
    Truncated     bool                   `json:"truncated"`      // window > retention
    TotalTokens   int64                  `json:"total_tokens"`
    EstimatedCost *float64               `json:"estimated_cost,omitempty"` // nil when pricing unknown
    CostCurrency  string                 `json:"cost_currency,omitempty"`
    CostStatus    string                 `json:"cost_status,omitempty"`    // reuse session_usage semantics (contract/session_usage.go)
    Days          []OverviewUsageDay     `json:"days"`
    AgentShare    []OverviewAgentShare   `json:"agent_share"`    // 30d fixed, descending, tail rolled into "other"
}
type OverviewUsageDay struct { Date string `json:"date"` Tokens int64 `json:"tokens"` }
type OverviewAgentShare struct { AgentName string `json:"agent_name"` Tokens int64 `json:"tokens"` Fraction float64 `json:"fraction"` }

type OverviewPulsePayload struct {
    Buckets   []OverviewPulseBucket `json:"buckets"`   // 7×24, zero-filled
    Busiest   *OverviewPulseBucket  `json:"busiest,omitempty"`
    LongestSession *OverviewLongestSession `json:"longest_session,omitempty"`
    TopTool   *OverviewTopTool      `json:"top_tool,omitempty"`
}
type OverviewPulseBucket struct { Weekday int `json:"weekday"` Hour int `json:"hour"` Events int `json:"events"` }
type OverviewLongestSession struct { SessionID string `json:"session_id"` AgentName string `json:"agent_name"` DurationSeconds int64 `json:"duration_seconds"` Date string `json:"date"` }
type OverviewTopTool struct { ToolID string `json:"tool_id"` Calls int `json:"calls"` }

type OverviewNetworkPayload struct { MessagesToday int `json:"messages_today"` }
type OverviewSystemPayload  struct { HookRunsToday int `json:"hook_runs_today"` HookFailuresToday int `json:"hook_failures_today"` RetentionDays int `json:"retention_days"` }
```

Insight fields are pointers: when the window has no data they are omitted and the UI drops the insight (honest empty, never a fake stat).

### 5.2 Aggregation layer (`internal/observe` + `internal/store`)

New `internal/observe/overview.go` — `QueryOverview(ctx, OverviewQuery) (Overview, error)` composing store calls; unit-tested against seeded stores like `tasks_dashboard` tests.

New store queries (sqlc, `internal/store/globaldb/queries/`), all workspace-filtered:

| Query | Shape | Feeds |
|---|---|---|
| `TaskRunOutcomesByDay` | `GROUP BY date(ended_at)` over task runs, terminal statuses, `since` 14d | Outcomes, Today KPIs |
| `TasksClosedByDay` | `GROUP BY date(completed_at)` over tasks | Today KPIs |
| `EventCountsByHourWeekday` | `GROUP BY strftime('%w'), strftime('%H')` over event summaries, `since` 14d | Pulse |
| `ToolCallCountsSince` | tool-call event summaries grouped by tool id, `since` 14d | Top tool |
| `NetworkMessagesSince` / `HookRunsSince` | event summaries by type family, `since` today (daemon-local day boundary) | Network/System counters |
| `TokenUsageByDay` | per-day token totals, `since` window | Usage chart + KPI |
| `TokenUsageByAgent` | token totals joined session→agent, `since` 30d | Agent share |
| `SessionRuntimeMaxSince` | longest session runtime in window | Pulse insight |
| `SessionAgentMetricsSince` | extend `AggregateSessionsByAgent` (`internal/store/session_catalog_agent_metrics.go:39`) with an optional `since` bound | Agents zone 7d window |

**Schema-risk flag (resolve first, Phase 1):** daily token bucketing requires token usage rows carrying timestamps. If `token_stats` rows are per-session cumulative without per-day provenance, add a usage-sample/rollup table via **`agh-schema-migration`** (declarative schema + gap-free goose migration + `atlas.sum` + sqlc via `make codegen`). Any new index (e.g., event summaries `(workspace_id, timestamp)`) follows the same skill. If neither is acceptable for Phase 1, `usage.days` ships empty with `cost_status:"unavailable"` and the chart renders its honest empty state — the KPI then shows session-summed totals only (`AggregateTokenStatsCost`, `internal/store/token_stats_cost.go:30`).

Cost estimation reuses the session-usage pricing path (model-catalog pricing; same `cost_status`/`cost_source` semantics as `contract/session_usage.go:6`). Footnote copy in UI: "Cost is estimated from model-catalog pricing and may diverge from provider billing."

Attention block composition (server-side, so CLI sees the same list): inbox lanes via `taskpkg.InboxReader` (`internal/observe/task_inbox.go:31`) → `approval` items from lane `approvals`, `failure` from `failed_runs`, `needs_input` from `blocked` where `blocking_reason` is an input/question class. `actions` emitted strictly from what the daemon accepts for the item's current status (§3 rule 6).

### 5.3 Wiring & co-ship (mandatory, one change)

Per `agh-contract-codegen-coship` and the codegen pipeline (`cmd/agh-codegen`, `magefiles/codegen.go:12`):

1. DTOs in `internal/api/contract/observe_overview.go`.
2. Handler `ObserveOverview` in `internal/api/core/` (new file, follows `task_read_handlers.go:222` pattern).
3. Routes in **both** `internal/api/httpapi/routes.go` (observe group, near `:111`) and `internal/api/udsapi/routes.go` (near `:130`).
4. OpenAPI operation + schemas in `internal/api/spec/registry_*` (observe registry).
5. CLI: `agh observe overview [-o json|table] [--workspace] [--usage-window]` — new `internal/cli/observe_overview.go` + client method; JSON output is the raw `ObserveOverviewPayload`.
6. `make codegen` regenerates `openapi/agh.json`, `web/src/generated/agh-openapi.d.ts`, `sdk/typescript/src/generated/contracts.ts`; `make codegen-check` gates drift.

### 5.4 Existing endpoints reused (no changes)

`/api/status`, `/api/observe/tasks/dashboard`, `/api/observe/tasks/inbox`, `POST /api/tasks/:id/approve|reject` (`core/task_read_handlers.go:282,326`), `/api/agents/catalog` (+`since` param per §5.2), `/api/network/status`, `/api/workspaces/:id/network/peers`, `/api/workspaces/:id/network/usage`, `/api/logs`, `/api/logs/stream`, `/api/sessions/catalog-stream`.

### 5.5 Freshness & live updates

No new SSE channel. The dashboard invalidates via existing streams (client-side, §6.4): session catalog stream (`core/session_catalog_stream.go:15`), logs stream (`core/logs_stream.go:18`), task stream events (`internal/events/registry.go:55-86` — `task.approved/rejected/needs_attention`, `task.run_*`). Overview responses carry `freshness` (reuse `TaskDashboardFreshnessPayload`) so the topbar Live/stale pill stays honest.

---

## 6. Frontend work

### 6.1 Location & structure (new domain `web/src/systems/dashboard/`)

Follows the per-domain convention (adapters / lib / hooks / components):

```
web/src/systems/dashboard/
├── adapters/overview-api.ts          # apiClient.GET("/api/observe/overview") etc.
├── lib/query-keys.ts                 # dashboardKeys.{overview,attention,…}
├── lib/query-options.ts              # overviewOptions(workspaceId, usageWindow) — staleTime 15s, refetch 30s (constants shared with tasks: lib/query-options.ts:37-40)
├── lib/activity-classes.ts           # quiet-event classification (tool calls, config reads, memory compaction) keyed off event `type` from internal/events registry
├── hooks/use-home-dashboard.ts       # composes overview + status + task dashboard + network + agents catalog + logs
├── hooks/use-home-live.ts            # stream subscriptions → cache invalidation
├── hooks/use-elapsed-ticker.ts       # 1s shared ticker for runcard elapsed
├── lib/home-prefs-store.ts           # zustand persisted: usageWindow, systemOpen (keys: agh-home-win / agh-home-sys)
└── components/                       # one file per zone component (≤500 lines each, split before writing)
```

`web/src/systems/os/apps/dashboard/dashboard-window.tsx` becomes a thin shell: topbar slot + `<HomeDashboard/>`. **Delete targets (zero-legacy):** `OverviewSection`, `DaemonStatusSection`, `METRIC_ORDER` (`dashboard-window.tsx:21-160`), old `use-dashboard-page.ts` shape (rewrite + rewrite its test).

Preload: extend `preloadHomeWorkspace` (`web/src/routes/_app/-app-preload.ts:44`) + `preloadDashboard` (`app-registry.tsx:93`) to warm `overviewOptions`, task dashboard, agents catalog, network status via `settleRouteQueries` (non-blocking).

### 6.2 Component map — prototype → production

Zone components are domain-prefixed (`Home*`) per the UI-reuse rule; anything generic gets promoted to `packages/ui` (story + test) instead of being forked.

| Prototype element | Production component | `@agh/ui` primitives used |
|---|---|---|
| `w2head` (44px, glyph + Live + New session) | OS window head + `useTopbarSlot` (already the pattern — `dashboard-window.tsx:35-40`) | `ConnectionIndicator`, `Button` (primary, opens the canonical start-session modal) |
| `pagemeta` | `HomePageMeta` | `Time`, plain text — 12px `text-subtle`, hairline bottom |
| `section__label` (+count, +act link) | `Section` label slot + count chip + trailing link | `Section`, `Eyebrow`, count via `--spacing-count-chip` chip, `PillLink`/`Button ghost sm` |
| `panelbox` | **`Panel`** — promote `tasks-dashboard-panel.tsx` into `packages/ui/src/components/custom/panel.tsx` (title/meta/right/body/foot slots, flat `bg-canvas-soft`, `radius-lg`); tasks dashboard migrates to it in the same change (hard cut, delete the tasks-local copy) | new primitive + story + test |
| Zone 1 `att__row` | `HomeAttentionRow` | `StatusDot` (warning/danger), `Button --sm` primary/ghost, `Time`; resolved state → `success-tint` row |
| Zone 2 KPI tiles | `HomeKpiStrip` | `MetricGrid columns=4` + `Metric labelCase="eyebrow"` + `SlidingNumber` (count-up, motion-based, reduced-motion aware) + `Sparkline` (usage tile) |
| Zone 3a `runcard` | `HomeRunCard` | `Pill.Dot tone="accent" pulse`, `OwnerAvatar`/initial chip, `KindChip` ("task run"), elapsed via `use-elapsed-ticker` + `formatDuration` (`packages/ui/src/lib/format-time.ts`) |
| Zone 3b network `prow`/`netbudget` | `HomeNetworkPanel` | `Panel`, `StatusDot`, `MonoId`, `Progress` (budget meter), foot link pinned `margin-top:auto` |
| Pulse heatmap | `HomePulseHeatmap` — CSS grid (34px + 24 cols), cells alpha-ramped on `--color-viz-cell`; **no chart lib** | `Panel`, `Tooltip` (cell hover), insights row uses `MonoId` |
| Outcomes chart | `HomeOutcomesChart` — recharts stacked `BarChart` (lazy, same pattern as `queue-health-sparkline.tsx:46-79`): segments `success` / `viz-other` / `danger`, rx 2, first/last date ticks, hover tooltip | `Panel`, recharts via dynamic import, legend with 8px swatches |
| Usage chart + window pills | `HomeUsageChart` — recharts `AreaChart` (lazy): `viz-line` stroke 1.5, `viz-fill` area, 3 gridlines, crosshair tooltip; window `PillGroup` (7d/30d/90d) persisted; retention-truncation footnote | `Panel`, `PillGroup`, recharts |
| Per-agent share | `HomeAgentShare` | `StackedProgress` (neutral lightness ramp segments) + legend |
| Zone 5a `agrow` table | `HomeAgentsPanel` — grid rows `1.1fr 52px 64px 52px 1fr`, sticky-free, runtime bar = `Progress` neutral | `Panel`, `OwnerAvatar`, mono numerics (`tabular-nums`), zero values `text-faint`, failed>0 `text-danger` |
| Zone 5b activity feed | `HomeActivityFeed` — modeled on `activity-feed.tsx` (network) row anatomy; tone dots; "Earlier today" separator; quiet-events `Collapsible` ("N quieter events — tool calls and config reads") | `StatusDot`, `Collapsible`, `Time`, `Skeleton` rows |
| Zone 7 `sysbar` + `tiles` | `HomeSystemPanel` — collapsed one-line bar (dot + "All systems normal" + mono summary) → `Collapsible` detail with 6 `MetadataTile`s + CLI hint `code-block` (`agh observe overview -o json`) | `Panel`, `Collapsible`, `MetadataTile`, `StatusDot` |
| `tip` tooltip | recharts `Tooltip` content styled `bg-elevated border-line shadow-overlay`; non-chart hovers use `Tooltip` | `Tooltip` |

Typography/geometry fidelity: KPI value `--text-kpi-value` (24px) at `--font-weight-display` (620); section labels `--text-section-head` uppercase; mono numerics `tabular-nums`; panel radius `--radius-lg`; page column `max-w-[1240px]` with `px-9 pt-6 pb-20` (within `--container-content-max`).

### 6.3 Data hooks (queries)

| Hook | Endpoint | Options |
|---|---|---|
| `useOverview(workspaceId, usageWindow)` | `/api/observe/overview` | staleTime 15s, refetch 30s |
| `useTaskDashboard` (existing) | `/api/observe/tasks/dashboard` | reuse `taskDashboardOptions` (`web/src/systems/tasks/lib/query-options.ts:126`) |
| `useTaskInbox` (existing) | `/api/observe/tasks/inbox` | approvals actions via `use-task-actions.ts` mutations |
| `useAgentsCatalog(since=7d)` | `/api/agents/catalog` | existing options + new param |
| `useNetworkStatus` / `useNetworkUsage` / `useNetworkPeers` (existing) | network routes | poll 30s (no network SSE) |
| `useHomeActivity` | `/api/logs?workspace&limit=30` | staleTime 5s; live via logs stream |
| `useDaemonHealth` (existing) | status | System zone + Live pill |

Workspace scoping: all keys include the active workspace id (`useActiveWorkspace`); home/global workspace maps to global scope exactly like `taskScopeForActiveWorkspace` (`web/src/systems/tasks/lib/workspace-scope.ts`).

### 6.4 Live updates (`use-home-live.ts`)

EventSource-per-domain pattern (no shared hook exists — follow `use-task-stream.ts`):

- **Session catalog stream** (`/api/sessions/catalog-stream`) → invalidate overview KPIs + working-now + agents.
- **Logs stream** (`/api/logs/stream?workspace`) → prepend/invalidate activity; throttled invalidate of pulse (≥60s).
- **Task events** (via logs-stream event types `task.approved`, `task.rejected`, `task.needs_attention`, `task.run_completed/failed/…` — `internal/events/registry.go:55-86`) → invalidate attention, task dashboard, outcomes/today.
- Approve/reject mutations optimistically resolve the attention row (prototype's resolved state: `success-tint` row, "Approved — X is starting"), then invalidate inbox + overview.

### 6.5 States

Every panel wraps in `DataSurface` (loading/empty/error/content — `packages/ui/src/components/custom/data-surface.tsx`):

- **Loading**: `Skeleton`/`SkeletonRows` mirroring final geometry (KPI tiles, 3 runcards, chart block heights) — extend the `TasksDashboardLoadingSkeleton` approach.
- **Empty (honest)**: Needs you → "Nothing needs you right now" (panel stays, quiet); Working now → "No agents working — start a session" + secondary CTA; charts with no data → `Empty` inside panel, axis-free; insights with null fields simply don't render.
- **Error**: per-panel `Empty icon=AlertTriangle` with retry; fatal daemon disconnect keeps the existing window-level branch (`dashboard-window.tsx:53-66`).
- **Stale**: `freshness.stale` → topbar Live pill flips to stale tone (existing `use-os-attention.ts` pattern); body never renders its own stale banner.

### 6.6 Interactions (fidelity checklist from prototype JS)

1. KPI count-up 550ms ease-out (`SlidingNumber`), skipped under reduced motion.
2. Runcard elapsed tickers — single shared 1s interval, `mm:ss` / `Hh MMm` format.
3. Approve → optimistic row resolve + counters decrement (attention count chip, Needs-you KPI + sub-label recompute).
4. Quiet-events fold — `aria-expanded`, chevron rotate, count in label.
5. Usage window pills — `aria-pressed`, persisted, re-renders chart + totals + truncation footnote.
6. System fold — persisted; summary line stays mono/truncating.
7. Chart hovers — outcomes per-day tooltip (completed/failed/canceled), usage crosshair + dot + per-day cost, heatmap per-cell events count.
8. Every zone/CTA navigates: KPIs → sessions/inbox/tasks/usage routes; runcards → session/run detail; agents rows → agent detail; activity rows → source entity; foot links → domain routes. No dead ends.
9. Focus-visible rings on every interactive element (`--shadow-focus-ring`); charts carry `role="img"` + composed `aria-label` (as in prototype).

### 6.7 Responsive

Prototype breakpoints, translated to Tailwind: `grid-2` collapses <1080px; KPIs 4→2→1 (<1080 / <520); attention row drops time column <760px; agents table hides sessions/failed columns <760px; heatmap horizontal-scrolls (`min-width 640px`). The OS mobile mode (<960px fullscreen windows) already wraps this — the page itself must not introduce horizontal scroll except the heatmap wrap.

---

## 7. `packages/ui` changes (promotions — with story + test each)

1. **`Panel`** (new custom primitive): title/meta/right/body/foot slots, flat variant per §6.2. Tasks dashboard migrates; `tasks-dashboard-panel.tsx` deleted (hard cut).
2. **`Sparkline`**: confirm current CSS-bar variant renders the KPI line-spark acceptably; if a line variant is added, it lives here (SVG path, `viz-line`/`viz-fill`), not in `web/`.
3. No other primitive gaps: `Metric`, `MetricGrid`, `StackedProgress`, `StatusDot`, `Pill`, `PillGroup`, `KindChip`, `Progress`, `MetadataTile`, `Collapsible`, `DataSurface`, `SlidingNumber`, `Empty`, `Skeleton`, `Tooltip`, `Time`, `MonoId`, `OwnerAvatar` cover the rest (inventory: `packages/ui/src/index.ts`).

Shadowing any `@agh/ui` export in `web/` is a blocked lint error (`compozy-ui-reuse/no-shadow-ui-primitive`) — domain components take `Home*` names.

---

## 8. Libraries

**No new dependencies.** Everything needed is present:

| Need | Library | Where |
|---|---|---|
| Charts (outcomes bars, usage area) | `recharts ^3.9.2` — always lazily imported (React.lazy + Suspense), per `queue-health-sparkline.tsx:46` | `packages/ui` dep |
| Count-up / micro-motion | `motion ^12.42.2` (via `SlidingNumber`) | `web` dep / ui peer |
| Dates | `date-fns ^4.4.0` + `format-time.ts` helpers | `web` |
| Data | `@tanstack/react-query ^5.101.3`, `openapi-fetch` typed clients (`web/src/lib/api-client.ts:13-21`) | `web` |
| Prefs store | `zustand ^5.0.14` (persist) | `web` |
| Heatmap | none — CSS grid | — |

Explicitly rejected: d3/visx (recharts is the repo standard), react-countup (SlidingNumber exists), any tooltip lib (ui `Tooltip` + recharts).

---

## 9. Phased delivery

**Phase 1 — Backend overview (blocks everything NEW)**
Resolve the token-timestamp schema question (§5.2 flag) → store queries (+ migration if needed) → `observe.QueryOverview` + unit tests → contract DTO + handler + HTTP/UDS routes + spec registry + CLI verb → `make codegen`. *Gate:* `agh observe overview -o json` returns truthful data on a seeded home; `make codegen-check` clean.

**Phase 2 — Frontend data layer**
`systems/dashboard/` adapters, keys, options, `use-home-dashboard`, `use-home-live`, prefs store, preload wiring. *Gate:* hook-level tests (mock client) for composition, invalidation, workspace scoping.

**Phase 3 — Zones: Needs you · KPIs · Working now · Network**
`Panel` promotion (+ tasks migration + delete), attention rows with real approve/reject, KPI strip, runcards, network panel. *Gate:* interactions §6.6 items 1–3; flush-bottom rule on row 1.

**Phase 4 — Charts: Pulse · Outcomes · Usage/share**
Lazy recharts components, heatmap, window pills, tooltips, honest empties. *Gate:* §3 rules 4–5; interactions 5, 7.

**Phase 5 — Agents · Activity · System + polish**
Remaining zones, responsive pass, reduced motion, a11y labels, skeletons, delete-target sweep. *Gate:* `agh-ui-screenshot` capture cited vs. prototype (Visual Contract Mode: rendered reference/implementation bundle, zero unresolved structural mismatches); `make verify`.

Each backend phase task carries the `Web/Docs Impact` subitem; the two-touch rule applies (a third patch to the same behavior becomes a redesign spec).

---

## 10. Testing contract (placement per `consolidate-test-suites`)

| Invariant | Layer | Suite |
|---|---|---|
| Overview aggregates correct per seeded fixtures (day buckets, windows, retention truncation, workspace scoping) | Go unit | `internal/observe/overview_test.go` (pattern: existing `tasks_dashboard` tests) |
| Store GROUP BY queries (incl. empty windows, timezone/day boundary) | Go unit | store suite for the new queries |
| Contract/spec parity for the new endpoint (HTTP+UDS) | Go | `internal/api/httpapi` parity + `internal/api/spec` tests (existing patterns) |
| CLI `observe overview` output shape | Go | `internal/cli` suite |
| `Panel` primitive render/slots | Vitest + story | `packages/ui` |
| Attention approve flow (optimistic resolve + counter decrement + invalidation) | Vitest | `web/src/systems/dashboard/__tests__/` |
| Dashboard composition states (loading/empty/error/ready) | Vitest | rewrite `use-dashboard-page`/window tests |
| Home route still mounts dashboard window + preloads | existing router integration test | `web/src/routes/_app/__tests__/` |

Coverage floor 80% per package; no snapshot/prose tests.

---

## 11. AGH Impact Audit

- **Native tools:** no impact — no `agh__*` tool IDs, toolsets, descriptors, schemas, or capability gates change; checked the tool registry surfaces while adding only an observe read endpoint + CLI verb.
- **Extensibility and hooks:** new read-only endpoint `/api/observe/overview` (HTTP+UDS) + CLI `agh observe overview` extend the agent-manageable surface; no hook events added or changed (existing `task.*`/`network.*`/hook event families are only *read*). Config lifecycle: reads the existing observability retention key; no new config keys.
- **Workspace data isolation:** every new aggregate is workspace-scoped (`workspace` param; global/home scope explicit); store queries filter `workspace_id`; SSE-driven cache invalidation keys include workspace id — tests in §10 assert no cross-workspace leakage in day buckets, agent share, and attention items.
- **Official AGH skill:** update `skills/agh/` — document `agh observe overview` and the overview read model (public CLI/API surface change).

**Web/Docs impact:** `web/` — this spec (routes `_app/index`, `systems/os/apps/dashboard`, new `systems/dashboard`, `packages/ui` Panel + tasks dashboard migration). `packages/site` — document the home dashboard + `observe overview` CLI/API in the runtime docs; screenshots refresh.

**QA tracker:** new user-visible behavior → add `untested` content-addressed scenarios under `docs/qa/scenarios/` (home dashboard zones, approve-from-home, usage window persistence, CLI overview); flag, don't retest.

---

## 12. Open decisions (UNCONFIRMED — resolve in Phase 1, do not guess)

1. **Token timestamps** (§5.2): per-day usage needs timestamped usage rows — confirm `token_stats` provenance; choose rollup-table migration vs. Phase-1 "unavailable" fallback.
2. **Retry verb**: confirm a run-retry/run-again endpoint exists for `failure` attention items; otherwise ship `open` only.
3. **Scheduler "next wake"**: confirm `StatusPayload.automation` exposes it; otherwise the Scheduler tile shows status only.
4. **Storage tile**: DB/disk size is absent from status — optional `OverviewSystemPayload` extension or drop the tile (prototype ships Retention instead; keep Retention, defer storage).
5. **Messages today** (§4 #9): ship windowed counter vs. relabel.
