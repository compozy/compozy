# CompozyOS Glossary

Canonical vocabulary for CompozyOS. When the corpus is ambiguous (older RFC drafts, older `.codex/ledger` entries, internal Slack/notes), this document is authoritative.

---

## Core Concepts

### CompozyOS and `compozy`

**CompozyOS** is the public product name used in ordinary prose, UI, package descriptions, calls to action, and formal category language. It names the complete agent operating system: the daemon-owned runtime, sessions and work model, memory, permissions, automation, OS shell, extensibility, and coordination.

**`compozy`** is the command identifier. The binary, `COMPOZY_*` environment variables, Go module path, `@compozy/*` packages, Homebrew formula, socket names, config paths, and `compozy__*` native tool IDs keep this spelling. CompozyOS is the product; `compozy` is its command. Do not use `CompozyOS Runtime` as a separate product name; `CompozyOS runtime` is descriptive prose when the runtime specifically matters.

### Daemon

The local background runtime process. It owns state, executes work, and serves every control surface. `daemon` is the canonical word in code, config, CLI, specs, and runtime docs.

**UI label:** "CompozyOS" (for example, "CompozyOS is running"). Never "daemon" on an end-user surface.

---

### ACP

**Agent Client Protocol** — the standard CompozyOS uses to talk to agent CLIs, spoken over JSON-RPC/stdio to a spawned subprocess. CompozyOS is an ACP host; Claude Code, OpenClaw, and Hermes are ACP-compatible agent CLIs it drives.

**UI label:** none. Expand the acronym on first use in end-user prose; reference and protocol docs may use `ACP` directly.

---

### Session

A durable managed agent run: saved history, resumable state, and the same view from CLI, API, and web. Prefer `session` over `chat`.

**UI label:** "session" — already everyday English. Gloss it on first use rather than aliasing it.

**Derived sessions** (lineage `kind`; copy rules in `COPY.md` §6 "Session Continue & Fork Terms"):

- **continue** — a new session for another agent, runtime, or declared route with a session's conversation carried over; the source stays unchanged. UI: "Continue with another agent…", pill/divider "Continued from …".
- **fork** — a conversation fork: a second session with the same agent and the conversation up to a point. UI: "Fork session…", "Fork from here", "Forked from …". Unqualified `fork` means this; worktree and Loop forks are always qualified.
- **recovery** — an empty restart of a session whose runtime cannot be resumed. UI: "Restart in a new session" — never "fork".
- `handoff` stays the Network term; it never labels continue or fork in UI copy (the `provider_error.next_action` value `handoff` is a wire value, not a label).

---

### Profile

An operator-owned partition of work on one CompozyOS installation. Each work root — session, task, loop run, automation, automation run, worktree, notification cursor, tool approval grant, usage row — is stamped with exactly one profile at creation and never moves. Children inherit their parent's owner. Profile-aware owned-work reads run in one of exactly two modes — scoped to one profile, or the explicit owner-labeled aggregate — and the daemon enforces both, fail-closed. Ruled exceptions are documented, not implicit: worktrees are visible in every profile with an owner tag.

Every installation has the permanent `default` profile, structurally identical to any other, which cannot be archived, deleted, or renamed. Names match `^[a-z][a-z0-9-]{0,31}$`, are unique across active and archived profiles, and reserve `default`, `all`, and `global`. The name doubles as a repository folder name and as the binding key extensions place resources into; the stamp itself is a stable ULID, so a rename never touches work.

**Organization, not access control.** Profiles carry no permission, grant, lock, or hidden-from vocabulary. Any active profile is selectable at any time; archived profiles remain visible in aggregate reads and must be unarchived before selection. Do not reintroduce the closed [Cross-workspace access](#cross-workspace-access) vocabulary. Access control for more than one operator is a separate future program.

**Bare "Profile" names only this concept.** The pre-existing compounds keep their own meaning and are unrelated: [Task Execution Profile](#task-execution-profile), layout profile, gateway connection profile, and session creation profile. Renaming them was considered and rejected; this entry is the arbitration line for any future collision.

**UI label:** "Profile" — keep. It has no alias, so it has no row in [Surface Names](#surface-names).

---

### Control surface

A human/agent-operable surface — CLI, HTTP/SSE, UDS, or web UI — over the same daemon-owned state. The term names the class, not any one surface.

**UI label:** none — internal vocabulary. It stays the runtime term in specs and docs and never reaches an end-user label.

---

### Capability

An authored description of an outcome an agent can perform. Optional capability catalogs live
beside `AGENT.md` and supply agent inspection, task eligibility, and reviewer selection.
The runtime normalizes the catalog and computes a digest; the agent name and capability ID identify
the offer. A capability describes an outcome; a [Loop](#loop) executes a deterministic program.

Use `capability` consistently in code, docs, and CLI. Extension `capabilities.provides` names
runtime interfaces; extension permissions name Host API access. Those are separate concepts.

---

### Provides

The closed set of **runtime interfaces an extension implements**, declared as `capabilities.provides` in the extension manifest and generated from the SDK declaration by `compozy extension build`.

Public set: `tool.provider`, `memory.backend`, `model.source`, `loop.watch_source`, `view.provider`, `connectivity.provider`, `forge.provider`. `view.provider` is the TypeScript-only programmable command-palette interface (`view/open`, `view/event`, `view/close`).

Each provide binds the extension to the CompozyOS → extension service methods the daemon will call (for example `memory.backend` → `memory/store`, `memory/recall`, `memory/forget`). Validation is closed-set membership, not shape: an unknown value fails manifest load rather than loading as a silent no-op.

**Say:** "provide surface", "the extension provides `tool.provider`". **Do not say:** "the extension's capabilities" when you mean this — see the [Capability](#capability) disambiguation.

---

### Permissions

The single authored list of **Host API methods an extension may call**, declared as `permissions.requires` in the extension manifest.

The list is validated against the closed Host API method set at build, validate, install, and daemon load. CompozyOS **derives** the operator-facing consent areas from it (`sessions:read`, `memory:write`, …) — consent areas are a display and policy projection, never an authored field.

Enforcement is per call against the effective grant, which is the declared list narrowed by the install source tier. Published sources (`curated`, `github`, `git`) run under the marketplace ceiling; local-path installs and dev links carry no ceiling.

**Not to be conflated with:** agent permission modes (`[permissions] mode` in `config.toml`, which governs tool approval).

---

### Extension Kit

The static resources shipped by one extension: skills, agents and their sidecars, Loops, automation, layouts, and MCP sidecars. Marketplace and local installs leave the kit inert; enable publishes resources owned by that extension instance, and disable removes them. The bundled `spec-cycle` extension is enabled on a fresh CompozyOS home, while later boot and update paths preserve the operator's explicit enabled or disabled state. Use **extension kit**, never **Bundle**, for this product concept.

### Extension Secret Binding

An instance-scoped link from a manifest-declared environment key to an existing Vault reference. Public reads expose only bound key names, never values or references.

### Gateway Requirement Confirmation

Explicit consent from an operator or authorized agent to the exact digest of an extension's normalized gateway permissions. Enable or update must receive the daemon-returned digest; the selected Gateway tier must match a declared permission.

---

### Loop

The **deterministic runtime program the daemon owns and executes**, defined by the contract **goal → act → verify → stop** plus a fixed set of named terminal outcomes (ADR-001). A Loop's body is a static DAG of typed nodes; iteration is simply what happens when verification says "not done." A single-pass linear pipeline is still a Loop — one that finished on its first pass — and it still carries the contract (definition-of-done, verification gate, terminal states, budget), which is the value no plain DAG engine delivers.

Loops ride CompozyOS's existing durable foundations (work queue, sessions, automation, memory) — they are **not** a second execution engine. The serialized definition is `compozy.loop/v1` YAML; the resolved form is what the coordinator runs.

**Loop vs Capability:** a Loop is deterministic and runtime-owned; a capability describes an agent outcome.

**Terminal outcomes:** `done`, `no-op`, `blocked`, `failed`, `exhausted`, `stalled`, `canceled`. Live states: `queued`, `running`, `watching`, `needs-approval`, `paused`.

**Not to be conflated with:** the historical "workflow" positioning. CompozyOS is a runtime with a Loop domain.

**UI label:** "Loop" — **pending owner decision, do not alias.** `workflow` is no longer a forbidden synonym for [capability](#capability), but the historical positioning above is still warned off, so no alias lands until the owner decides.

---

### Skill

A **bundled procedural instruction** that a CompozyOS session can activate before doing work. Skills are local to CompozyOS (loaded via `internal/skills`), governed by `metadata.compozy.*` frontmatter, scanned via `VerifyContent`, and may declare MCP servers and lifecycle hooks.

**Skills vs. Capabilities:** skills guide behavior; capabilities describe the outcomes an agent offers.

**UI label:** "Skill" — keep the word; it already reads plainly. Gloss it on first use.

---

### Command palette identifiers

**Command palette** is the prose name for the daemon-canonical command catalog and invocation surface.
Use `cmd_palette` for Go packages, config and event families; `cmd-palette` for CLI verbs and URL-facing
slugs; and `compozy__cmd_palette_*` for native tool IDs. These spellings name one registry, not separate
features.

---

### Cross-workspace access

An agent session reaching a [workspace](#workspace) other than its own. It is a **boundary anchored on the session's effective `PermissionMode`**, evaluated by `internal/workspaceaccess` at the agent-identity, task, native-tool, spawn, and workspace-coordination seams: `approve-all` allows, `deny-all` denies with no prompt, `approve-reads` prompts at the native-tool seam and denies at every other seam.

**Not a grant, toggle, capability level, or trust list.** No `[workspace_access]` config section, grants table, CLI verb, native toolset, or Settings surface exists — ADR-007 removed them from the design and none was implemented. Do not reintroduce that vocabulary.

Prompt answers `allow_session`/`reject_session` are **session consent**: in-memory, session-scoped, applied at every seam, cleared when the session stops, with no management surface. Policy evaluations use the best-effort audit event types `workspace.access_granted` / `workspace.access_denied`.

`deny-all` is deliberately asymmetric: "ask for everything" on the tool-risk axis, "never cross" on the workspace axis.

Operator access is not cross-workspace access — operators are not governed by this policy, and the web deep-link workspace switch is operator UX, not an agent path.

---

### AGENT.md (frontmatter format)

Self-contained agent definition: YAML frontmatter (provider/model/tools/permissions) + Markdown prompt. The frontmatter `name` is trimmed and must match `^[a-z][a-z0-9_-]{0,105}$` (maximum 106 characters); filesystem-loaded definitions must also match their directory name. The current runtime portability unit is the CompozyOS agent directory rooted at `$COMPOZY_HOME/agents/<name>/` for global scope and `.compozy/agents/<name>/` for workspace or additional roots. That directory can carry agent-scoped `skills/` and other sidecars owned by the agent.

**Status:** Partially shipped from RFC 001. The runtime now parses `AGENT.md` frontmatter,
including agent-local `skills/` overlays and `skills.disabled`. Draft fields such as
`skills.inherit`, `skills.extra_sources`, and `memory.*` remain out of scope today.

**vs AGENTS.md (project file)**:

- `AGENTS.md` = project-level instructions (industry convention, plain Markdown).
- `AGENT.md` = single-agent definition (CompozyOS proposal, structured frontmatter).
- Different filenames, different purposes. Do not conflate.
- The standardization path (extension to AGENTS.md under AAIF, vs. standalone) is open per RFC 001 §6.6.

---

## Identity

### Caller Identity (operational)

Inside CompozyOS, agent-facing CLI commands resolve identity from `COMPOZY_SESSION_ID` / `COMPOZY_AGENT` through `internal/agentidentity`. **Operator endpoints MUST NOT infer agent identity from environment variables.** Agent → identity-implicit. Operator → identity-explicit.

---

## Memory

### Memory Types (taxonomy)

Per RFC 002 / Claude Code AutoDream / CompozyOS `internal/memory/consolidation/`:

- `user` — persona, role, preferences, knowledge.
- `feedback` — rules and corrections from past interactions.
- `project` — context about ongoing work, who/why/by-when.
- `reference` — pointers to where info lives in external systems.

### Memory Scopes

- `agent` — local to a specific agent definition; only this scope accepts `agent_tier = workspace | global`.
- `workspace` — shared across agents and across profiles within a workspace; repository-committed.
- `profile` — shared across workspaces, owned by one [Profile](#profile). Stored under `$COMPOZY_HOME/profiles/<name>/memory/`.

The scope value `global` was hard-cut to `profile`; no dual value is accepted. `global` survives only as an **agent tier**, which is a different axis (how far one agent's memory reaches, not who owns it).

Default write scope is declared per agent in `memory.scope`.

### Consolidation Gates (cascade by cost)

**Time Gate** (default 24h since last consolidation) → **Session Gate** (default 3 sessions touched) → **Lock Gate** (`tryAcquireConsolidationLock` to prevent multi-instance races). All must pass. Never replace with naive heuristics.

---

## Autonomy

### Background role

A named daemon-owned responsibility routed through the closed `[roles]` roster: `coordinator`,
`dream`, `checkpoint_summary`, `memory_extractor`, `auto_title`, or `memory_controller`. Empty
session-role agents resolve either to a virtual builtin (`coordinator` or `dreaming-curator`) or to
the invoking context (`memory_extractor` and `auto_title`); the memory controller is an in-process
model call and has no agent identity.

`[roles]` owns routing — enabled state, agent/provider/model/reasoning selection, ordered fallbacks,
and the small amount of policy inseparable from coordinator sessions or controller calls. The
owning subsystem keeps its operational policy, gates, scoring, cadence, and prompts. Background
roles do not replace or govern Loop DSL model defaults, `TaskExecutionProfile`, or automation
resources.

### `task_run`

The single durable work-queue row. Carries `claim_token`, `lease_until`, `heartbeat_at`, the owning `session_id`, and the run's execution identity. **Never duplicated by a parallel queue.**

### `claim_token` / `claim_token_hash`

Opaque, fenced ownership token. Raw `claim_token` (`compozy_claim_*`) NEVER appears over the wire, in logs, in SSE, in web UI, in channel messages, or in memory. Public form is `claim_token_hash`.

### `ClaimNextRun(criteria)`

The single authoritative claim primitive. Lives in `internal/task`. The mechanical scheduler does NOT call it.

### Coordinator

A managed CompozyOS session whose semantic role is to orchestrate executable workspace runs. Auto-spawn
is conservative: a run must be enqueued, coordinator auto-start enabled, no healthy active
coordinator present, and spawn caps available.

### Mechanical Scheduler

Daemon-owned operational-safety component (`internal/scheduler`). Idle registry, capability-aware wakeups, lease sweep, recovery, backpressure. **Does not claim runs.** Wake/observe/sweep are advisory.

### Task Execution Profile

The task-owned typed overlay that selects the runtime shape of orchestration for one task. Persisted under `task_execution_profiles` plus selector side tables (never in `metadata_json`). Configured under `[task.orchestration.profile]` and managed through `compozy task profile inspect|update|delete`, `/api/tasks/{id}/profile`, native task tools, and the web UI Task setup sheet.

The profile carries `CoordinatorProfile` (`mode = "inherit" | "guided"`), `WorkerProfile` (worker agent/provider/model + worker eligibility selectors), `ReviewProfile` (reviewer selectors), `ParticipantPolicy` (allowed/preferred agents and capabilities), `WorktreePolicy`, and `RuntimePolicy`. Validation runs at write time in `task.Service.SetExecutionProfile`; session start loads the persisted profile without re-running validation. PUT replaces the entire profile — omitted blocks normalize to defaults.

The profile is **not** runtime authority: task ownership remains in `task_runs`, worker mutation remains session-bound, review verdict authority remains `task.Service.RecordRunReview`, execution policy does not bypass tool/approval policy, and coordinator guidance does not create queue or terminal-state authority.

### Notification Cursor

The shared durable delivery-progress primitive in `internal/notifications`. Identity is `(scope_kind, workspace_id, consumer_id, stream_name, subject_id)`; storage is `notification_cursors`. Every identity component and delivery id is an opaque, valid UTF-8 value preserved byte for byte. The cursor records `last_sequence`, `last_delivery_id`, `last_delivered_at`, `last_error`, and `updated_at`.

Advance is monotonic; same-sequence replay is accepted only when both sequence and delivery id match. `Reset` is the only path that may lower a cursor and requires an explicit recovery reason. Cursors do **not** assign tasks, claim runs, complete runs, replace SSE replay cursors, or replace task hooks. Notification cursors are NOT SSE `after_sequence` cursors — SSE cursors are client-side replay positions, while notification cursors are daemon-side confirmed-delivery state.

### Run Review

The post-terminal review attached to a `task_run` in `task_run_reviews`. Created by `task.Service.RequestRunReview` (idempotent on `(run_id, review_round, attempt = 1)`), bound to a reviewer session by `BindRunReviewSession`, and persisted by `task.Service.RecordRunReview` (the sole verdict authority). Run review status is `requested | routed | in_review | recorded | circuit_opened | canceled`. Verdict outcomes (orthogonal to status) are `approved | rejected | blocked | error | timeout | invalid_output`. `approved` and `rejected` are not statuses.

### Continuation Run

A new `task_run` enqueued by `task.Service.RecordRunReview` when a `rejected` verdict still has `max_rounds` remaining, linked by `task_runs.review_id` and replayed by delivery id. Carries reviewer-supplied `missing_work` and `next_round_guidance`. Continuation runs use the task's current `TaskExecutionProfile` at enqueue time; they do not rewrite the previous run.

### Task Context Bundle

The shared rendered overlay assembled by `internal/situation`, exposed in Go as `task.ContextBundle` and on the wire as `/agent/context.task.bundle`. Carries run summary, continuation guidance, review history, redacted active-run context, reviewer-bound context, and `latest_event_seq` projection. Reviewer sessions can receive a review-bound context bundle without receiving a worker lease — context implies neither claim ownership nor mutation rights.

### Current Run ID

`tasks.current_run_id` is a denormalized read projection over `task_runs`, maintained only by `task.Service`/store transition methods. It is **not** claim authority, scheduler assignment authority, coordinator ownership authority, or terminal-state authority. API and web payloads expose it as read-model state. Profile mutation rejects while `current_run_id` is non-empty.

### Safe Spawn

Daemon-managed child-session creation. Defaults: `max_depth = 1`, `max_children = 5`, mandatory TTL. Permission narrowing on **concrete atoms only**: tools, skills, MCP server IDs, and workspace path grants. Subset-only; unknown child atoms count as widening and reject.

---

## OS Shell

The web UI presents as a desktop environment: a menubar, persistent virtual desktops, tiled and floating windows, and a dock. These terms name that presentation model. The runtime object remains the `workspace`; switching a desktop never changes runtime scope.

### Workspace

The runtime object: project root and scoped runtime context (sessions, memory, tasks, vault, config). A workspace owns window-manager topology, but remains the unit of runtime scope. The Workspaces surface switches this runtime context.

**UI label:** "project". The alias may NOT be `environment` — that word is reserved for process-level context. `workspace` stays canonical in code, `workspace_id`, CLI, API, and docs.

### Desktop

One persistent virtual arrangement inside a workspace. A workspace may own multiple ordered desktops. Each desktop owns tiled groups and floating-window order; a window belongs to exactly one desktop. The active desktop and focused window are client-local projections. A desktop carries no sessions, memory, or tasks of its own.

### Tiled group

One non-overlapping layout tree inside a desktop. A desktop may contain multiple tiled groups and floating windows. A tiled group uses leaf, split, and stack nodes; resizing acts on a structural split boundary, not on independent window rectangles.

### Window

A frame hosting one app's durably resumed route subtree. The window head is the route's `<Topbar>` with OS controls injected. Windows are presentation containers; the views inside them are the same `systems/*` views the routes render. A window may be tiled, stacked, or floating; structural placement is distinct from its internal pathname/search route.

### Desktop pager

The minimal horizontal dot control in the menubar tray for switching the active client's desktop; an orange dot marks an off-screen desktop that needs you. It is navigation, not a persistent management panel. The tray's All desktops button opens Desktops Overview, which owns create, rename, reorder, transfer, and delete operations.

### Dock

The rail of app launchers along the left edge, below the menubar (a bottom tab bar in compact presentation). It mirrors the app inventory and carries running/focused/minimized indicators and badges bound to runtime projections (waiting sessions, awaiting-approval tasks). Its foot holds the profile switcher, the light/dark toggle, and Settings.

### Menubar

The top bar across the full window: CompozyOS mark (whose menu holds Settings), Global scope globe, workspace trigger, app menus, then the tray — desktop pager, All desktops, the approvals bell, and the ⌘K palette. The globe sits between the mark and the chip and is the only owner of Global vs workspace destination. The chip reads the project name when scoped down, or **Global** (`~`) when Global scope is on.

### Window manager

The daemon-authoritative, workspace-scoped topology and semantic command surface for desktops, tiled groups, and windows. Durable mutations use revision checks and atomic commits. Browser geometry projection, active desktop, focus, and gesture previews remain client-specific where defined by the contract.

**Window manager vs memory:** window-manager data is *presentation topology* interpreted by the daemon. `memory` is *agent* knowledge (see Memory above). Window-manager documents hold no agent knowledge, and memory holds no window geometry.

### Terminal

The daemon-owned command surface for visible, interactive, or supervised work. Use **Terminal** as the product name; never use **console** or **shell pane** as aliases. Provider-internal command execution remains session activity and is not a Terminal.

---

## Compatibility

### Compatibility regime

Which of the three SD-013 rules governs a changed surface, decided by who owns it: **user state** never breaks, **public surfaces** deprecate before they delete, **internal code** hard-cuts. The surface lists live in `CLAUDE.md` §Compatibility Policy and SD-013.

### Stability label

`stable` or `experimental`, applied to a public surface. Every surface shipped in a tagged release is `stable` unless its docs and CLI help say `experimental`; only `experimental` surfaces may change shape without a deprecation window.

### Deprecation window

The one release during which a public surface's old shape still works beside the new one and emits a warning naming the replacement. The old shape is deleted in the release after. Never longer, never stacked (N-2 is never accepted).

### Boundary shim

The translation that implements a deprecation window or an auto-migration: a config-loader alias, an HTTP/UDS decoder rule, a CLI verb-table entry, or a Goose migration. Lives at the edge, never as an `if oldShape` branch in domain code, and its comment and release note name the release that removes it.

---

## Verification & Testing

### `make gate`

Required local pre-push gate. It maps the branch diff to affected lint, test, codegen, and tooling lanes; it never runs the full monorepo gate implicitly.

### PR CI / `make gate-full`

PR CI is the required full delivery gate at the exact head SHA. `make gate-full` is the optional local form and retains the machine-wide capacity lock.

### `make codegen` / `make codegen-check`

Regenerate / verify drift on `openapi/compozy.json`, `web/src/generated/compozy-openapi.d.ts`. Mandatory after any `internal/api/contract` change.

### Test Layers

- **Unit** (`make test`) — fast, race-enabled, per package.
- **Integration** (`make test-integration`) — `+integration` build tag, co-located.
- **E2E Runtime** (`make test-e2e-runtime`) — daemon-side Go harness against `acpmock`.
- **E2E Web** (`make test-e2e-web`) — Playwright against the daemon-served SPA.
- **E2E Nightly** (`make test-e2e-nightly`) — heavy E2E, runs in release-PR `dry-run` job only.

### Real-Scenario QA

The practice of validating CompozyOS the way real users experience it, owned by the `qa-report` (planning, living docs) + `qa-execution` (persona-driven sessions, evidence) pair over the committed `docs/qa/` tree (`state.csv`, `bugs/BUG-NNNN` registry, journeys, charters, dated reports). For release-grade validation of the multi-agent runtime, the `eng-real-scenario-qa` skill adds the playbook harness: an isolated lab (via `eng-qa-bootstrap`), one in-persona operator kickoff, read-only runtime observation, and a strict deliverable/collaboration audit — exercising CLI + Web + API surfaces end-to-end.

---

## "What CompozyOS Is Not"

For positioning consistency on the marketing site and in docs:

- CompozyOS is **not an MCP replacement**. MCP integrates _into_ CompozyOS skills via `metadata.compozy.mcp_servers`.

---

## Surface Names

Aliases are UI labels, never renames. `capability` remains the only name in code, wire, docs, and CLI. Forbidden synonyms stay forbidden.

Canonical values stay in code, payloads, CLI, API, and reference docs; the alias is a UI label only. The canonical noun stays reachable one step deeper — tooltip, detail view, or inspector. An alias that appears in a config key, a wire payload, or a generated reference is a rename, and renames do not happen through this table.

This table mirrors the Surface Aliases table in `COPY.md` §6. The two are one table in two places: a change to either requires the same change to the other. This file owns the reservations; `COPY.md` owns the surface rules that use them.

| Canonical | UI surface alias | Notes |
| --- | --- | --- |
| `daemon` | "CompozyOS" / "CompozyOS is running" | Never "daemon" in end-user UI. |
| `workspace` | "project" | Alias may NOT be "environment" — reserved for process-level variables and operating-system context . |
| `event ledger` | "history" | Use only where the implementation actually exposes the event trail. |
| `tool registry` / `toolset` | "what agents are allowed to do" | A descriptive gloss, not a label swap. |
| `control surface` | — drop from UI | Internal vocabulary; stays the runtime term in specs and docs. |
| `capability` | keep + define on first use | Wire identity `(peer_id, capability_id)` unchanged. `recipe`, `procedure`, and `playbook` stay forbidden everywhere. |
| `session` | keep + gloss on first use | Already everyday English. |
| `terminal` | "Terminal" | First-class product surface. Never label it "console" or "shell pane". |
| `Loop` | — pending owner decision | Do not alias. `workflow` is released as a forbidden synonym for `capability`, but the historical "workflow" positioning is still warned off (see [Loop](#loop)). |
| `Jobs` / `Triggers` (dock titles) | — pending owner decision | Do not rename. |
| settings group `Operator` | "Personal" | Group label only. |
| settings section `Observability` | "Diagnostics" | Section label only. |
| settings section `Attention` | "Notifications" | Section label only. |
| settings section `Gateway` | "Remote access" | Section label only. |
| settings group `workspace` | "Basics" | Group label only. |
| settings group `runtime` | "Agents" | Group label only. |
| settings group `system` | "Advanced" | Group label only. |
| Loop `generation` | "round" | One iteration of a Loop run. Wire, CLI, and payloads keep `generation`. |
| Loop step `quarantined` | "set aside" | A step removed from scheduling after repeated failures; the UI verb is "Retry" (wire: requeue). |
| fork (built-in Loop) | "Copy and edit" | UI verb for forking a built-in Loop into the project. |
| memory `dream` | "tidy up" | Memory consolidation. `dream` stays in API, CLI, and config keys. |
| extension dev overlay | "local development copy" | Menu verb "Unlink local copy". |
| session status tokens (`waiting-for-input`, `hung`, `unhealthy`, …) | "Needs your answer", "Stuck", "Having trouble", … | Display words only; the token stays on `data-badge`/aria. Color only states that need the user. |

`Roles`, `Hooks`, and `Extensions` keep their names — glossary terms that already read plainly.

---

## Style

- File names: kebab-case for code/config, snake_case for memory files.
- Identifiers in code: Go conventions (`PascalCase` exported, `camelCase` unexported).
- Capability/skill IDs: kebab-case.
- Commit prefixes: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `build:` only.
