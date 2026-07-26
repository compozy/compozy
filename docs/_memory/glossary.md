# AGH Glossary

Canonical vocabulary for AGH and AGH Network. When the corpus is ambiguous (older RFC drafts, older `.codex/ledger` entries, internal Slack/notes), this document is authoritative.

---

## Core Concepts

### Capability

The single canonical name for **reusable agent artifacts** that describe transferable delegation offers, network discovery shapes, and AGH artifacts shipped between peers.

A capability is **interpretive**, not deterministic — it tells an agent what is available, not how to execute a deterministic program.

**Forbidden synonyms:** `recipe` (used in pre-rename RFC 003-old), `procedure`, `playbook`. If you find these in code, docs, or task artifacts targeting current behavior, rename them. `workflow` is no longer a forbidden synonym: the runtime now owns a first-class **Loop** domain (see below, ADR-001). A capability is still never a workflow/loop — but the word `workflow` no longer implies a naming collision.

**Source:** RFC 003-v0 (`.../agh-rfcs-local/003-agh-network-v0.md`) renamed `recipe` → `capability`. RFC 004 enforces.

**Operational identity:** `(peer_id, capability_id)`.

**Capability vs Loop:** a capability is the interpretive network artifact (what an agent offers to peers); a [Loop](#loop) is the deterministic runtime program the daemon owns and executes. The network carries capabilities, never loop execution.

---

### Loop

The **deterministic runtime program the daemon owns and executes**, defined by the contract **goal → act → verify → stop** plus a fixed set of named terminal outcomes (ADR-001). A Loop's body is a static DAG of typed nodes; iteration is simply what happens when verification says "not done." A single-pass linear pipeline is still a Loop — one that finished on its first pass — and it still carries the contract (definition-of-done, verification gate, terminal states, budget), which is the value no plain DAG engine delivers.

Loops ride AGH's existing durable foundations (work queue, sessions, automation, network, memory) — they are **not** a second execution engine. The serialized definition is `agh.loop/v1` YAML; the resolved form is what the coordinator runs.

**Loop vs Capability:** a Loop is deterministic and runtime-owned; a [capability](#capability) is interpretive and network-shipped. Loops do not replace capabilities, and loop execution never travels over the network wire.

**Terminal outcomes:** `done`, `no-op`, `blocked`, `failed`, `exhausted`, `stalled`. Live states: `queued`, `running`, `watching`, `needs-approval`, `paused`.

**Not to be conflated with:** the historical "workflow" positioning. AGH is a runtime with a Loop domain; the AGH Network protocol remains not a workflow engine.

---

### Skill

A **bundled procedural instruction** that an AGH session can activate before doing work. Skills are local to AGH (loaded via `internal/skills`), governed by `metadata.agh.*` frontmatter, scanned via `VerifyContent`, may declare MCP servers and lifecycle hooks.

**Skills vs. Capabilities:** Skills live inside an AGH instance and govern an agent's behavior locally. Capabilities cross AGH instances over the network and describe what an agent offers to peers. A skill could be exposed as a capability, but they are not the same artifact.

---

### Sandbox

The AGH execution boundary selected for a workspace or session. A sandbox profile is configured under `[sandboxes.<name>]`, selected by `sandbox_ref` or runtime flags, and carried through the session lifecycle as sandbox metadata.

Implemented providers are `local` and `daytona`. Provider lifecycle surfaces use `sandbox.prepare`, `sandbox.ready`, `sandbox.sync.before`, `sandbox.sync.after`, and `sandbox.stop` hooks, plus the extension Host API methods `sandbox/list`, `sandbox/info`, and `sandbox/exec`.

Do not call this product feature an `environment`. Reserve `environment`, `env`, and `environment variable` for process-level variables and operating-system context.

---

### AGENT.md (frontmatter format)

Self-contained agent definition: YAML frontmatter (provider/model/tools/permissions) + Markdown prompt. The current runtime portability unit is the AGH agent directory rooted at `$AGH_HOME/agents/<name>/` for global scope and `.agh/agents/<name>/` for workspace or additional roots. That directory can carry agent-scoped `skills/` and other sidecars owned by the agent.

**Status:** Partially shipped from RFC 001. The runtime now parses `AGENT.md` frontmatter,
including agent-local `skills/` overlays and `skills.disabled`. Draft fields such as
`skills.inherit`, `skills.extra_sources`, and `memory.*` remain out of scope today.

**vs AGENTS.md (project file)**:

- `AGENTS.md` = project-level instructions (industry convention, plain Markdown).
- `AGENT.md` = single-agent definition (AGH proposal, structured frontmatter).
- Different filenames, different purposes. Do not conflate.
- The standardization path (extension to AGENTS.md under AAIF, vs. standalone) is open per RFC 001 §6.6.

---

### Peer Card

The AGH Network discovery artifact: a peer's identity and `peer_card.capabilities` index, optionally
with `peer_card.ext["agh.capabilities_brief"]` for AGH-specific projection.

**vs A2A Agent Card:** A2A Agent Cards are an external industry standard. Peer Cards are AGH-Network specific but could be generated FROM an AGENT.md definition (RFC 001 §3.3 is open on the mapping). Today they are not unified.

---

## Identity

### `peer_id`

A network-scoped identifier matching `[a-z0-9][a-z0-9._-]{0,127}`.

### `nickname@fingerprint`

The verified-format identity in AGH Network v1. `nickname` matches `[a-z0-9_-]{1,32}`; `fingerprint` is the first 32 lowercase hex of `SHA-256(pubkey)`.

**Critical:** A `nickname@fingerprint`-shaped identity arriving WITHOUT a valid `proof` MUST classify as `rejected`, NOT `unverified`. This is the proof-stripping defense from RFC 004 §3.3.

### Caller Identity (operational)

Inside AGH, agent-facing CLI commands resolve identity from `AGH_SESSION_ID` / `AGH_AGENT` through `internal/agentidentity`. **Operator endpoints MUST NOT infer agent identity from environment variables.** Agent → identity-implicit. Operator → identity-explicit.

---

## Network Wire (RFCs 003-v0, 004)

### Message Kinds (MVP allowlist)

The six canonical core kinds: `greet`, `whois`, `say`, `capability`, `receipt`, `trace`.

Message kinds describe what happened. They do not describe where the message lives.

### Conversation Surfaces

Conversation-bearing messages use `surface` to declare where they live:

- `surface:"thread"` for public-thread messages.
- `surface:"direct"` for direct-room messages.

`greet` and `whois` are discovery messages and must not carry a conversation surface.

### `public_thread`

A public N-to-N conversation container inside one `channel`.

Wire shape:

- `surface:"thread"`
- `thread_id`

Public threads are visible to peers with access to the channel. A public thread can contain ordinary chat,
capability transfers, and zero or more lifecycle-bearing work units.

### `direct_room`

A restricted two-party conversation container inside one `channel`.

Wire shape:

- `surface:"direct"`
- `direct_id`

Direct rooms restrict default runtime visibility to the two room peers plus operator/audit access. They are not
cryptographic privacy and do not imply end-to-end encryption.

### `work_id`

Lifecycle-bearing work inside exactly one conversation container. `work_id` is not a conversation identifier,
task-run identifier, route key, claim token, or queue lease.

Receipts and traces require `work_id`. Ordinary `say` and `capability` messages carry `work_id` only when they
open or continue lifecycle-bearing work.

For coordination channels (autonomy MVP): `status`, `request`, `reply`, `blocker`, `handoff`, `result`, `review_request`.

Future-RFC kinds explicitly NOT in MVP: `contract-net`, `multi-home`, `vote`, `react`, `escalate`, `offer`, `accept`, `decline`, complex mention routing.

### Lifecycle States

`submitted → working → needs_input → completed | failed | canceled`. Post-terminal regression is forbidden.

### Cancellation Duality

- `receipt(canceled)` = initiator-side withdrawal.
- `trace(canceled)` = worker-side abort.

### Trust States

`verified` / `unverified` / `rejected`. Default classification for non-conformant proofs is `rejected` (not `unverified`).

RFC 004 signed content includes `surface`, `thread_id`, `direct_id`, and `work_id` when present. A receiver must
verify canonical bytes before injecting defaults.

### Replay Defense

Bounded replay window via `id`. Recommended 300-second clock-skew rejection when `expires_at` is null.

### Trust Profile

**Baseline Trust Profile** (RFC 004): Ed25519 + RFC 8785 JCS + SHA-256 fingerprints. Profile id `agh-network.trust.ed25519-jcs/v1`. Self-certified handles only — no DIDs, no revocation, no organization-level authorization, no federation policy in this profile.

---

## Memory

### Memory Types (taxonomy)

Per RFC 002 / Claude Code AutoDream / AGH `internal/memory/consolidation/`:

- `user` — persona, role, preferences, knowledge.
- `feedback` — rules and corrections from past interactions.
- `project` — context about ongoing work, who/why/by-when.
- `reference` — pointers to where info lives in external systems.

### Memory Scopes

- `agent` — local to a specific agent definition.
- `workspace` — shared across agents within a workspace.
- `global` — shared across workspaces in the AGH installation.

Default write scope is declared per agent in `memory.scope`.

### Consolidation Gates (cascade by cost)

**Time Gate** (default 24h since last consolidation) → **Session Gate** (default 5 sessions touched) → **Lock Gate** (`tryAcquireConsolidationLock` to prevent multi-instance races). All must pass. Never replace with naive heuristics.

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

The single durable work-queue row. Carries `claim_token`, `lease_until`, `heartbeat_at`, the owning `session_id`, and the execution's immutable resolved Network participation snapshot. A `run_kind = "network_wake"` row is taskless and identifies the wake, owner execution, and target session explicitly. **Never duplicated by a parallel queue.**

### `claim_token` / `claim_token_hash`

Opaque, fenced ownership token. Raw `claim_token` (`agh_claim_*`) NEVER appears over the wire, in logs, in SSE, in web UI, in channel messages, or in memory. Public form is `claim_token_hash`.

### `ClaimNextRun(criteria)`

The single authoritative claim primitive. Lives in `internal/task`. The mechanical scheduler does NOT call it.

### Coordinator

A managed AGH session whose semantic role is to orchestrate executable workspace runs. Auto-spawn
is conservative: a run must be enqueued, coordinator auto-start enabled, no healthy active
coordinator present, and spawn caps available. Network participation is not a bootstrap condition.

### Mechanical Scheduler

Daemon-owned operational-safety component (`internal/scheduler`). Idle registry, capability-aware wakeups, lease sweep, recovery, backpressure. **Does not claim runs.** Wake/observe/sweep are advisory.

### Coordination Channel

The workspace-scoped conversation channel created only when a task run's immutable
`resolved_network_participation` snapshot is Live. Local runs create no channel. Explicit run intent
wins over the task execution profile, then workspace coordination, then built-in Local. Enabling
workspace coordination affects future runs only. Conversation evidence is never task ownership or
status authority.

### Task Execution Profile

The task-owned typed overlay that selects the runtime shape of orchestration for one task. Persisted under `task_execution_profiles` plus selector side tables (never in `metadata_json`). Configured under `[task.orchestration.profile]` and managed through `agh task profile inspect|update|delete`, `/api/tasks/{id}/profile`, native task tools, and the web UI Task setup sheet.

The profile carries `CoordinatorProfile` (`mode = "inherit" | "guided"`), `WorkerProfile` (worker agent/provider/model + worker eligibility selectors), `ReviewProfile` (reviewer selectors), `ParticipantPolicy` (allowed/preferred channels, peers, agents, capabilities), `SandboxPolicy` (`mode = "inherit" | "none" | "ref"`), and an optional `network_participation` request for future runs. Validation runs at write time in `task.Service.SetExecutionProfile`; session start loads the persisted profile without re-running validation. PUT replaces the entire profile — omitted blocks normalize to defaults.

The profile is **not** runtime authority: task ownership remains in `task_runs`, worker mutation remains session-bound, review verdict authority remains `task.Service.RecordRunReview`, sandbox policy does not bypass tool/approval policy, and coordinator guidance does not create queue or terminal-state authority.

### Notification Cursor

The shared durable delivery-progress primitive in `internal/notifications`. Identity is `(consumer_id, stream_name, subject_id)`; storage is `notification_cursors`. The cursor records `last_sequence`, `last_delivery_id`, `last_delivered_at`, `last_error`, and `updated_at`.

Advance is monotonic; same-sequence replay is accepted only when both sequence and delivery id match. `Reset` is the only path that may lower a cursor and requires an explicit recovery reason. Cursors do **not** assign tasks, claim runs, complete runs, replace SSE replay cursors, replace task hooks, or define bridge delivery targets. Notification cursors are NOT SSE `after_sequence` cursors — SSE cursors are client-side replay positions, while notification cursors are daemon-side confirmed-delivery state.

### Bridge Task Subscription

The delivery-target row in `bridge_task_subscriptions` that selects which bridge instance, task, delivery mode, and routing fields receive a terminal task notification. Owns target state only. Cursor identity is fixed to `consumer_id = "bridge_task_subscription:<subscription_id>"`, `stream_name = "task_events"`, `subject_id = <task_id>`; delivery progress lives in the matching `notification_cursors` row.

Subscription delete removes the active target row only. Stale cursor diagnostics remain inspectable by cursor key, and same-id recreation resumes from the preserved cursor. Public route shape is `/api/tasks/{id}/notifications/bridges` (create/list) and `/api/tasks/{id}/notifications/bridges/{subscription_id}` (show/delete) across HTTP, UDS, OpenAPI, generated TypeScript, CLI, and generated CLI docs.

### Run Review

The post-terminal review attached to a `task_run` in `task_run_reviews`. Created by `task.Service.RequestRunReview` (idempotent on `(run_id, review_round, attempt = 1)`), bound to a reviewer session by `BindRunReviewSession`, and persisted by `task.Service.RecordRunReview` (the sole verdict authority). Run review status is `requested | routed | in_review | recorded | circuit_opened | canceled`. Verdict outcomes (orthogonal to status) are `approved | rejected | blocked | error | timeout | invalid_output`. `approved` and `rejected` are not statuses.

### Continuation Run

A new `task_run` enqueued by `task.Service.RecordRunReview` when a `rejected` verdict still has `max_rounds` remaining, linked by `task_runs.review_id` and replayed by delivery id. Carries reviewer-supplied `missing_work` and `next_round_guidance`. Continuation runs use the task's current `TaskExecutionProfile` at enqueue time; they do not rewrite the previous run.

### Task Context Bundle

The shared rendered overlay assembled by `internal/situation`, exposed in Go as `task.ContextBundle` and on the wire as `/agent/context.task.bundle`. Carries run summary, continuation guidance, review history, redacted active-run context, reviewer-bound context, and `latest_event_seq` projection. Reviewer sessions can receive a review-bound context bundle without receiving a worker lease — context implies neither claim ownership nor mutation rights.

### Current Run ID

`tasks.current_run_id` is a denormalized read projection over `task_runs`, maintained only by `task.Service`/store transition methods. It is **not** claim authority, scheduler assignment authority, coordinator ownership authority, or terminal-state authority. API and web payloads expose it as read-model state. Profile mutation rejects while `current_run_id` is non-empty.

### Safe Spawn

Daemon-managed child-session creation. Defaults: `max_depth = 1`, `max_children = 5`, mandatory TTL. Permission narrowing on **concrete atoms only**: tools, skills, MCP server IDs, workspace path grants, network channels, env profile grants. Subset-only; unknown child atoms count as widening and reject.

---

## OS Shell

The web UI presents as a desktop environment: a menubar, persistent virtual desktops, tiled and floating windows, and a dock. These terms name that presentation model. The runtime object remains the `workspace`; switching a desktop never changes runtime scope.

### Workspace

The runtime object: project root and scoped runtime context (sessions, memory, tasks, vault, config). A workspace owns window-manager topology, but remains the unit of runtime scope. The Workspaces surface switches this runtime context.

### Desktop

One persistent virtual arrangement inside a workspace. A workspace may own multiple ordered desktops. Each desktop owns tiled groups and floating-window order; a window belongs to exactly one desktop. The active desktop and focused window are client-local projections. A desktop carries no sessions, memory, or tasks of its own.

### Tiled group

One non-overlapping layout tree inside a desktop. A desktop may contain multiple tiled groups and floating windows. A tiled group uses leaf, split, and stack nodes; resizing acts on a structural split boundary, not on independent window rectangles.

### Window

A frame hosting one app's durably resumed route subtree. The window head is the route's `<Topbar>` with OS controls injected. Windows are presentation containers; the views inside them are the same `systems/*` views the routes render. A window may be tiled, stacked, or floating; structural placement is distinct from its internal pathname/search route.

### Desktop pager

The minimal lower-left horizontal dot control for switching the active client's desktop, aligned with the Dock centerline. It is navigation, not a persistent management panel. Desktops Overview owns create, rename, reorder, transfer, and delete operations.

### Dock

The bottom strip of app launchers. It mirrors the app inventory and carries running/minimized indicators and badges bound to runtime projections (waiting sessions, awaiting-approval tasks).

### Menubar

The top bar across the desktop: workspace trigger, app menus, the approvals bell, the ⌘K palette, and Settings.

### Window manager

The daemon-authoritative, workspace-scoped topology and semantic command surface for desktops, tiled groups, and windows. Durable mutations use revision checks and atomic commits. Browser geometry projection, active desktop, focus, and gesture previews remain client-specific where defined by the contract.

**Window manager vs memory:** window-manager data is *presentation topology* interpreted by the daemon. `memory` is *agent* knowledge (see Memory above). Window-manager documents hold no agent knowledge, and memory holds no window geometry.

---

## Verification & Testing

### `make verify`

Blocking commit gate: `fmt → lint → test → boundaries → build`. Zero warnings, zero errors.

### `make codegen` / `make codegen-check`

Regenerate / verify drift on `openapi/agh.json`, `web/src/generated/agh-openapi.d.ts`. Mandatory after any `internal/api/contract` change.

### Test Layers

- **Unit** (`make test`) — fast, race-enabled, per package.
- **Integration** (`make test-integration`) — `+integration` build tag, co-located.
- **E2E Runtime** (`make test-e2e-runtime`) — daemon-side Go harness against `acpmock`.
- **E2E Web** (`make test-e2e-web`) — Playwright against the daemon-served SPA.
- **E2E Nightly** (`make test-e2e-nightly`) — heavy E2E, runs in release-PR `dry-run` job only.

### Real-Scenario QA

The practice of validating AGH the way real users experience it, owned by the `qa-report` (planning, living docs) + `qa-execution` (persona-driven sessions, evidence) pair over the committed `docs/qa/` tree (`state.csv`, `bugs/BUG-NNNN` registry, journeys, charters, dated reports). For release-grade validation of the multi-agent runtime, the `real-scenario-qa` skill adds the playbook harness: an isolated lab (via `agh-qa-bootstrap`), one in-persona operator kickoff, read-only runtime observation, and a strict deliverable/collaboration audit — exercising CLI + Web + API surfaces end-to-end.

---

## "What AGH Is Not"

For positioning consistency on the marketing site and in docs:

- **AGH Network is not a workflow engine.** Capabilities are interpretive, not deterministic programs, and envelopes never carry loop execution. (The AGH *runtime* does own a deterministic [Loop](#loop) domain — but it stays off the network wire, per ADR-001.)
- AGH is **not a federation protocol**. AGH Network v1 is a self-certified pairwise envelope, not a federated trust system.
- AGH is **not an MCP replacement**. MCP integrates _into_ AGH skills via `metadata.agh.mcp_servers`.
- AGH is **not an A2A replacement**. AGH Network is a peer-to-peer envelope; A2A is an industry standard. They can coexist.
- AGH **competes on runtime, SDK, observability, DX, and integration depth — NOT the open agent network protocol.** AGH Network must remain implementable outside AGH.

---

## Style

- File names: kebab-case for code/config, snake_case for memory files.
- Identifiers in code: Go conventions (`PascalCase` exported, `camelCase` unexported).
- Capability/skill IDs: kebab-case.
- Network channel IDs: lowercase alphanumeric with optional underscores or hyphens.
- Commit prefixes: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `build:` only.
