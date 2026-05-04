# Resources, Reconciliation, Extensions & Bridges — looper vs AGH

Scope: comparative audit of how the daemon manages declarative resources, reconciliation loops, extension wiring, and bridges to host/agent runtimes. Findings are filtered through looper's PRD / TechSpec / tasks workflow focus — AGH features that only apply to its agent-hosting/session platform are called out but not ported.

Repository anchors:

- looper: `/Users/pedronauck/Dev/compozy/looper`
- AGH:    `/Users/pedronauck/dev/compozy/agh`

---

## 1. Quick assessment — what looper already has correctly

### 1.1 Startup crash reconciliation (`internal/daemon/reconcile.go`)

- `ReconcileStartup` closes out every run still marked `starting`/`running` when the daemon restarts, appending a synthetic `run.crashed` event in per-run DB and updating the global DB in one batch via `globaldb.MarkRunsCrashed` (`reconcile.go:93-167`).
- Failure to rewrite a per-run DB is surfaced through `ReconcileResult.CrashEventFailures` and folded into a degraded health signal (`service.go:85-110`) rather than blocking readiness — a pragmatic choice that matches AGH's philosophy of isolating per-item failures from daemon liveness.
- The SQLite-header sanity check (`ensureSQLiteDatabaseFile`) guards against half-written per-run DBs, which AGH's `environment_reconcile.go` handles differently but with the same intent (skip unreadable session metadata).

### 1.2 Extension bridge (`internal/daemon/extension_bridge.go` + `internal/core/extension/daemon_bridge.go`)

- The `extensionBridge` type implements `extensions.DaemonHostBridge`, wiring extension-initiated child runs back into `RunManager.startRun` with per-mode (`ExecutionModePRDTasks`, `ExecutionModePRReview`, `ExecutionModeExec`) validation.
- Runtime config is normalized through `normalizeRuntime` before starting the child run: workspace root is inherited, `DaemonOwned=true` is forced, `TUI=false`, and the presentation mode is pinned to `detach`. This is the correct ownership model for a daemon-spawned run.
- Per-run scope injection happens in `RunManager.openRunScopeForStart` (`run_manager.go:1105-1126`), which attaches the bridge to the bootstrap context via `extensions.WithDaemonHostBridge` only when `EnableExecutableExtensions` is set — a clean opt-in.
- A random per-run capability token (`newExtensionHostCapabilityToken`) is generated to let child runtime validate callbacks against the daemon-host surface, which mirrors AGH's pattern of bounding extension authority.

### 1.3 Filesystem artifact watcher (`internal/daemon/watchers.go`)

- `workflowWatcher` has proper concurrency discipline: `stopCh`/`done` channels, one goroutine with `select { ctx.Done(), stopCh, watcher.Errors, watcher.Events, debounce.C }`, `sync.WaitGroup`-style flush-on-stop via `stopAndFlush`.
- Debouncing is correctly implemented (`watcherDebounce`) and flushes reliably on shutdown or context cancel.
- Directory add/remove is reconciled on every flush via `reconcileWorkflowWatches`, so newly created subdirs like `reviews-002/` or `memory/` are picked up without restarting the daemon.
- Artifact classification (`isRelevantWorkflowArtifact`) enumerates the exact filenames looper cares about: `_meta.md`, `_tasks.md`, `_prd.md`, `_techspec.md`, task `NNN.md`, `adrs/`, `memory/`, `qa/`, `prompt[s]/`, `protocol[s]/`, `reviews-N/`.
- Each change emits an `artifactSyncEvent` with a SHA-256 checksum so consumers can detect idempotent rewrites.

### 1.4 Extension runtime & chain dispatcher (`internal/core/extension/`)

- Extensions are loaded per-run via `Manager`, with hook chains frozen at construction time (`dispatcher.go`), a priority-ordered chain-of-responsibility for mutable hooks, and a fan-out pool for observer hooks. This matches AGH's dispatch shape.
- A review provider bridge (`review_provider_bridge.go`) is available for code-review extensions, with lazy session reuse and overlay registration via `provider.ExtensionBridge`. This is looper-specific functionality that AGH does not have (AGH has bridges, but for chat/IDE bridging).

**Bottom line:** looper's daemon already has the right building blocks for its workflow model: interrupted-run reconciliation, workspace artifact watching with debouncing, and a per-run extension bridge that enforces daemon ownership for spawned child runs. Nothing in the core mechanics is broken.

---

## 2. Gaps — concrete missing patterns

Priorities reflect *incremental value for looper's PRD/TechSpec/task workflow*, not the size of the port.

### GAP-1 — No durable extension registry / lifecycle service at the daemon level  *(Priority: Medium)*

**AGH reference:** `/Users/pedronauck/dev/compozy/agh/internal/daemon/extensions.go`

- `daemonExtensionService` (`extensions.go:17-64`) owns `List`, `Install`, `Enable`, `Disable`, `Status` on `udsapi.ExtensionService` with a shared `*extensionpkg.Registry` backed by the global DB.
- Every mutation (`Install`/`Enable`/`Disable`) calls `reload(ctx)` which both reinitializes the extension runtime and fans out `Sync` to downstream publishers (agent-skill, hook binding, tool-MCP, bundles) — see `reload` at `extensions.go:146-166`.
- `loadExtensionSnapshot` prefers the running runtime's live status (`ext.Status`) but falls back to the registry row when the extension is disabled, so `Status` returns meaningful data regardless of runtime state.

**What looper has today:** extensions are discovered and spawned *per run* inside `Manager.Start` (`manager.go:50-104`). There is no daemon-scoped registry, no `install/enable/disable` UDS surface, and no cross-run reload. The daemon only ever sees extensions as a context-scoped side-effect of `openRunScopeForStart` injecting `DaemonHostBridge`.

**Why it matters for looper:** the recent daemon migration (commit `ab0d26c`) makes the daemon a long-lived process. If a user installs or disables a review-provider extension (e.g., CodeRabbit adapter) while the daemon is running, today the change is only reflected on the next `compozy run` spawn. There is no API to list installed extensions, disable a misbehaving one, or force a reload without restarting the daemon. This becomes painful once more than one extension is in play (PR-review providers + custom hooks + SDK extensions).

**Concrete action — Adapt (not full port):**

- Add a lightweight `daemonExtensionService` in `internal/daemon/` backed by the existing `internal/core/extension` discovery/manifest code (`discovery.go`, `manifest_load.go`, `enablement.go`).
- Expose `List`/`Enable`/`Disable`/`Status` via the existing UDS transport (`sync_transport_service.go` or a new `extension_transport_service.go`).
- Keep the reload surface small: looper only needs one downstream subscriber today (the review provider overlay bridge in `review_exec_transport_service.go:195-231`). Signal it via a notifier callback rather than AGH's fan-out interface.

**Skip:** AGH's bundle/agent-skill/tool-MCP fan-out Sync, because looper does not have those resource systems.

---

### GAP-2 — No declarative resource store for agents or hook bindings  *(Priority: Low)*

**AGH reference:**
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/agent_skill_resources.go`
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/hook_binding_resources.go`
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/hook_bindings.go`

**Pattern:** AGH's `resources.Kernel` holds versioned records per `ResourceKind` (agent, skill, hook-binding, MCP-server, bridge-instance). Publishers (syncers) translate in-memory declarations into resource drafts; projectors apply committed revisions back to the live runtime. Every mutation triggers a projector reconciliation, and rollback compensation is implemented (see `bridges.go:941-1031`).

**Assessment for looper:** looper already ships agent definitions as bundled Go data (`internal/core/agents/catalog.go` area) and hook behavior lives inside extensions — there is no user-authored agent TOML or per-workspace hook binding to reconcile. Introducing `resources.Kernel` here would be heavy infrastructure with no user-visible payoff.

**Concrete action — Skip.** Revisit only if looper grows user-editable hook bindings or custom agent overlays per workspace. See §3.

---

### GAP-3 — Extension reload does not signal downstream subscribers  *(Priority: Medium)*

**AGH reference:** `bridges.go:1145-1167` (`reloadExtensions`), `extensions.go:146-166` (`reload` fan-out).

**What's missing:** looper's `ReviewProviderBridge` (`internal/core/extension/review_provider_bridge.go`) caches a session per workspace-and-command. If an extension manifest changes on disk (new version installed, capability list edited), the cached session will keep using the old subprocess until the bridge is explicitly recreated. `review_exec_transport_service.go:195-231` builds overlay bridges on every review run but does not invalidate them on extension manifest change.

**Why it matters for looper:** extension SDK users editing their manifest while `compozyd` runs get stale behavior. Even without a full registry (GAP-1), a minimal "extension-changed" notifier would fix this.

**Concrete action — Adapt (small):**

- Inside `internal/core/extension/manager.go`, add a `ManifestVersion()` or `ManifestChecksum()` accessor.
- In `internal/daemon/review_exec_transport_service.go` around `closeOverlayBridges`, invalidate the cached bridge when the manifest checksum differs from the one used at bridge creation.
- If GAP-1 is implemented later, replace this ad-hoc check with the registry-driven reload event.

---

### GAP-4 — Reconciliation emits no metrics/events beyond the one-shot summary  *(Priority: Medium)*

**AGH reference:** `environment_reconcile.go:516-541` (`environmentReconcileLogAttrs`), plus `environment_reconcile.go:58` logs `Info` with a structured session count.

**What looper does now:** `ReconcileStartup` returns a `ReconcileResult` but only `CrashEventFailures > 0` promotes a degraded health detail (`service.go:102-108`). There is no per-run log line, no event published to the global event journal, and `metrics` exposes only `runs_reconciled_crashed_total` (`service.go:120-128`).

**Why it matters for looper:** when 5+ runs crashed, operators need to know *which runs* were reconciled, not just how many. A user running `compozyd status` after a kernel panic has no way to see "run `task-32-abc` was marked crashed at 21:04" without grepping per-run DBs.

**Concrete action — Port (small):**

- After each `updates = append(...)` in `ReconcileStartup` (`reconcile.go:155-163`), emit a structured `slog.Info` via the daemon logger with `run_id`, `started_at`, `reconciled_at`, `duration_ms`, and whether the synthetic event was appended. Mirror the attribute shape of AGH's `environmentReconcileLogAttrs`.
- Consider publishing a durable event via the existing global event journal for each reconciled run (`pkg/compozy/events`) so `compozy logs` / `compozy runs watch` surface it.
- Extend `Service.Metrics` with counters for `runs_reconciled_total` (success) in addition to the current `runs_reconciled_crashed_total`, and a gauge for `last_reconcile_timestamp_seconds`.

---

### GAP-5 — Reconciliation is startup-only; no runtime drift check  *(Priority: Low)*

**AGH reference:** `environment_reconcile.go:30-59` is also startup-only, but AGH additionally reconciles during extension reload (`bridges.go:444-456`, `extensions.go:146-166`) and triggers typed-projector reconciliation on every resource mutation (`triggerBridgeResourceReconcile`).

**What looper does now:** the only reconciliation is at `Start`. If the global DB and a per-run DB drift while the daemon is alive (e.g., an operator force-kills a child exec process), the stale `starting`/`running` row is only cleaned up on the next daemon restart.

**Why it matters for looper:** the `RunManager.Shutdown` path and `activeRun.close` handle normal termination, but a SIGKILL on a spawned Claude Code / Droid child process leaks a `running` row until next boot. `compozyd run list` will show ghosts.

**Concrete action — Adapt:**

- Add a periodic scan (e.g., every 60s) that lists non-terminal runs in `globaldb`, checks whether their `RunManager.activeRuns[runID]` exists, and if absent marks them crashed. Reuse the existing `appendSyntheticCrashEvent` path from `reconcile.go:170-207`.
- This is a single goroutine with the standard `ctx.Done()` + `time.Ticker` pattern — no `resources.Kernel` needed.

---

### GAP-6 — Watchers do not carry a daemon-level "hot" resource view for TUI/API consumers  *(Priority: Low)*

**AGH reference:** `tool_mcp_resources.go:37-58` (`resourceCatalog`) — a typed, versioned in-memory snapshot that projectors push into and readers snapshot via `Snapshot()`. Paired with `composed_assembler.go` which composes multiple providers deterministically.

**What looper does now:** `workflowWatcher` emits sync events but there is no daemon-held "latest artifact index" — every API caller (`compozy tasks list`, TUI, review pipeline) re-reads disk.

**Why it matters for looper:** small — the disk reads are cheap and the TUI already tails the event journal. This is an optimization, not a correctness issue. However, if the TUI redesign (see `.codex/ledger/2026-04-20-MEMORY-task-run-tui.md`) exposes more real-time artifact previews, a central snapshot would avoid N-way reads.

**Concrete action — Skip for now, revisit with TUI work.** The `artifactSyncEvent.Checksum` field already carries enough info to cache-invalidate per-path.

---

### GAP-7 — No composed assembler pattern for prompt sections  *(Priority: Skip — but worth noting)*

**AGH reference:** `composed_assembler.go`, `section_selector.go`, `prompt_sections.go`, `prompt_input_composite.go`.

AGH composes system prompts from multiple sections (memory, skills, network) with budget-aware trimming, policy-based eligibility (`SectionPredicate`), and deterministic ordering. This is how chat sessions assemble a startup system prompt.

**Assessment for looper:** looper's prompts are built by `internal/core/prompt` using skill names + runtime context, and the actual prompt assembly lives inside each coding agent (Claude Code, Codex, etc.). Looper does not own a "system prompt" in the AGH sense — it feeds a task file path into the coding agent's own prompt machinery. The composed-assembler pattern does not apply.

**Concrete action — Skip.** See §3.

---

### GAP-8 — No hook binding materialization from workspace config  *(Priority: Low)*

**AGH reference:** `hooks_bridge.go:851-1032` assembles hook declarations from extensions, workspace config, and agent definitions via `chainDeclarationProviders`, deduplicated by the `hookBindingSourceSyncer` in `hook_bindings.go`.

**What looper does now:** extension hook chains are built per-run by `NewHookDispatcher(registry, audit)` (`dispatcher.go:48-53`) directly from the per-run extension registry. There is no workspace-level hook config (no `~/.compozy/hooks.toml` or per-workspace hook TOML), so there is nothing to materialize centrally.

**Concrete action — Skip.** Looper intentionally keeps all hook behavior inside extension manifests. If/when looper grows user-editable workspace hook bindings, revisit this. See §3.

---

## 3. Explicitly skipped — AGH patterns that do not apply to looper

| AGH pattern | Files | Why skip for looper |
|---|---|---|
| Bridge instances + secret bindings (chat/IDE adapters like Slack, Discord) | `bridges.go`, `bridge_resources.go`, `bridge_secrets.go` | Looper has no chat/IDE bridges. Its only "bridge" concept (review provider) is a single-shot subprocess invocation, not a long-lived connected bridge with secret storage. |
| Hook binding resource store + projector | `hook_bindings.go`, `hook_binding_resources.go`, `hook_agent_events.go` | Looper has no user-editable workspace hook config and no ACP agent-event streaming (hooks live inside extension manifests and fire via the dispatcher). Introducing `resources.Kernel` for hook bindings would be pure overhead. |
| Agent/skill resource sync | `agent_skill_resources.go` | Looper ships bundled agent definitions via `embed.go` — there is no per-workspace agent override layer. AGH's use case (marketplace skills + per-workspace overrides) does not exist here. |
| Bundle resources | `bundle_resources.go` | Looper does not package functionality as installable bundles; it has skills (bundled) and extensions. No bundle lifecycle exists to reconcile. |
| Automation jobs/triggers | `automation_resources.go` | Looper has no cron-like automation system. Workflows are user-initiated via CLI. |
| Tool/MCP server reconciliation | `tool_mcp_resources.go` | Looper does not expose MCP-server management. If MCPs are ever surfaced, AGH's `resourceCatalog[T]` is a reasonable reference but should not be copied preemptively. |
| Composed prompt assembler + section selector | `composed_assembler.go`, `section_selector.go`, `prompt_sections.go`, `prompt_input_composite.go` | Looper delegates final prompt composition to the coding agent (Claude Code, Codex, Droid). There is no startup-prompt injection surface to assemble. |
| Environment reconciliation (remote Daytona/sandbox sessions) | `environment_reconcile.go` | Looper has no remote execution environments — all runs are local subprocess fork/exec. The reattach/destroy lifecycle is irrelevant. |
| Session lifecycle fanout + hook telemetry fanout | `hooks_bridge.go:130-216` | Looper has no chat-session concept with create/resume/stop semantics. Runs are single-shot. |

---

## 4. Priority-ordered action summary

| # | Gap | Priority | Effort (rough) | Risk |
|---|---|---|---|---|
| GAP-1 | Daemon-scoped extension registry + UDS service (`list/enable/disable/status`) | Medium | ~300 LOC + tests | Low — opt-in feature, no change to existing run path |
| GAP-3 | Invalidate `ReviewProviderBridge` cache on manifest change | Medium | <80 LOC + test | Very low |
| GAP-4 | Reconciliation observability (per-run log, event, extended metrics) | Medium | ~120 LOC + test | Very low |
| GAP-5 | Periodic runtime drift scan for orphaned `starting`/`running` rows | Low | ~150 LOC + test | Low — needs careful interaction with `RunManager.Shutdown` |
| GAP-6 | Central artifact snapshot catalog | Low | Defer until TUI work | — |
| GAP-2 | Resource kernel / declarative agent + hook store | Low | — | Skip until user-editable resources exist |
| GAP-7 | Composed prompt assembler | Skip | — | Doesn't apply |
| GAP-8 | Hook binding materialization | Skip | — | Doesn't apply |

The highest-leverage wins are GAP-3 (trivial fix, visible to extension authors) and GAP-4 (makes daemon debuggable after crashes). GAP-1 is the natural next step once extensions become more central to looper's story.
