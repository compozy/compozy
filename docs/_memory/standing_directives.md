# Standing Directives

Ongoing engineering posture, not date-stamped per-task plans. These are perpetually active rules. Surfaced from undated entries in `.codex/plans/` and verified across the synthesis corpus.

---

## SD-001 — Long-Running Sessions Supervision

**Posture.** Compozy sessions can run for hours. Supervise activity, don't wait on wall-clock timeouts. Heartbeats, progress events, and idempotent cancel are the supervision primitives.

**Required behavior:**

- Activity supervisor per prompt/session inspired by Hermes; no wall-clock timeouts as the supervision primitive.
- Heartbeats update **metadata only** — never flow through the ACP event channel (backpressure risk). `runtime_progress` is a low-cadence persisted event.
- Warnings emit exactly once per session per cause; subsequent inactivity triggers cancel-with-grace before `StopTimeout`.
- `CancelPrompt`, session stop, and timeout collapse into ONE idempotent cancellation path.
- Inactivity timeout MUST NOT be implemented as a wall-clock timeout. Heartbeat must be positive; `0` disables warning/timeout/progress.
- Configuration under `[session.supervision]` with explicit zero-value semantics.

**Source:** `.codex/plans/long-running-sessions.md` (undated standing directive). Hermes Track 03 ACP & Session Lifecycle Hardening implements this.

**Triggers re-evaluation when:** any change to `internal/session/manager_*.go`, `internal/acp/client.go` lifecycle paths, or supervisor configuration is proposed.

---

## SD-002 — Remove Legacy Alpha Compatibility Code (SUPERSEDED 2026-09-04 by SD-013)

**Status.** Retired. Its premise — "a greenfield pre-1.0 beta with no production deployments to preserve" — stopped being true on 2026-08-22, when real users began running every release, and v0.3.0 shipped hard cuts that discarded their data (L-040). The internal-code half of SD-002 survives as regime 3 of SD-013: delete obsolete code, no `// legacy` branches or dual-naming inside Go / `web/` / `@compozy/ui`, hard-cut internal renames, and "composition-root `Reconcile` is current logic, not legacy" (SD-008). Its user-state and public-surface halves are replaced by regimes 1 and 2 of SD-013. Historical text: `.codex/plans/remove-legacy-alpha.md`.

---

## SD-003 — Conversation in BR-PT, Artifacts in English

**Posture.** Pedro types/speaks in Brazilian Portuguese; all persistent artifacts are English.

**Required behavior:**

- Respond in BR-PT when prompted in BR-PT.
- TechSpecs, ADRs, `_idea.md`, `_tasks.md`, code, tests, comments, commit messages, documentation, ledger files, memory files: English.
- Verbatim user quotes preserved in evidence/research artifacts may keep BR-PT (because they're evidence).
- BR-PT pushback markers ("fraco", "leviano", "ruim", "está totalmente errado", "meia boca", "esquecendo coisas") are escalation signals — slow down and re-clarify.

**Source:** `.codex/ledger/` notes; recurring pattern across all sessions.

---

## SD-004 — Multi-LLM Development Pipeline

**Posture.** Compozy development uses three LLMs with deliberate role assignment.

**Required behavior:**

- **Codex (`gpt-5.4` with `reasoning_effort=xhigh`)** authors TechSpecs, major Go code, autonomous-mode kernel work.
- **Claude Opus (`xhigh`)** pressure-tests TechSpecs in user-directed cross-LLM review rounds, reviews architecture decisions, writes/reviews React/E2E frontend code.
- **`gpt-5.4-mini` with `reasoning_effort=high`** runs as parallel subagents for breadth (codebase mapping, competitor analysis, conversation-log auditing) when explicitly delegated.
- Do not substitute models without explicit user approval.
- Subagents default to read-only — they return analysis to the parent agent, and the parent writes any required files. They may write/edit/commit only when the parent's prompt explicitly delegates that action; otherwise the parent authors the change. Skills with stricter contracts (e.g. `cy-spec-peer-review`, `deep-review`) keep their hard read-only rule for their dispatch lane.

**Source:** Direct quotes across many sessions; codified in `feedback_multi_llm_pipeline.md` (user memory).

---

## SD-005 — Real-Scenario QA Before Release

**Posture.** Automated verification (`make gate` locally plus exact-head PR CI) is necessary but not sufficient. Real-scenario QA against a multi-agent / multi-channel / multi-task workspace catches drift automated gates miss.

**Required behavior:**

- Every program ends with a `qa-report` task and a `qa-execution` task. UI-bearing features include browser-based e2e.
- QA state is living repo docs: the pair operates on the committed `docs/qa/` tree (`state.csv` scenario tracker, global `bugs/BUG-NNNN.md` registry, journeys, session charters, dated `reports/<date>-<scope>.md`). Per-round `qa/` trees and reset bug ids are the retired anti-pattern.
- For release validation on the multi-agent runtime, the `eng-real-scenario-qa` skill runs a playbook lab (bootstrap via `eng-qa-bootstrap`, one in-persona operator kickoff, runtime observation, strict audit) and delegates planning/execution mechanics to the `qa-report` + `qa-execution` pair; its findings also land in `docs/qa/`.
- Hermetic QA still respects each provider's auth contract: bound-secret and brokered lanes isolate `PROVIDER_HOME`, while `native_cli` providers with `home_policy=operator` keep the operator `HOME` / native login state unless the scenario explicitly validates isolated provider-home behavior.
- Concrete bug evidence (autonomy task_18 BUG-001/002/003, Hermes BUG-001..007) shows the QA pass surfaces real production bugs the unit/lint/build coverage cannot catch.

**Source:** Codex sessions (most-repeated request); `qa-report`/`qa-execution` SKILL.md (living-docs contract); `eng-real-scenario-qa` SKILL.md (runtime-observation harness); autonomy and Hermes QA verification reports (historical).

---

## SD-006 — Forensic-First Bug Fixes

**Posture.** Every bug-fix plan opens with a confirmed reproduction (timestamp, command, observed evidence) BEFORE listing changes. "I think" or "probably" at the top of a fix plan is forbidden.

**Required behavior:**

- Reproduce the bug with the narrowest real command before editing code.
- Record reproduction in the plan: timestamp, exact command, observed output.
- Distinguish symptom from root cause in writing.
- Fix at root cause; don't patch symptoms.
- Add focused regression coverage at the correct layer, or record why an existing gate already owns the invariant.
- Re-run the narrow reproduction, the impacted scenario, and relevant package tests.

**Source:** `.codex/plans/` (consistent forensic frame in `child-workgroup-activation.md`, `session-stop-hang.md`, `dashboard-xterm-visibility.md`, `prompt-stream-stall.md`); also encoded in `qa-execution` (fix-loop governor: regression test red-before/green-after), `eng-real-scenario-qa`, and `cy-fix-reviews`.

---

## SD-007 — Truthful UI > Plausible UI

**Posture.** UI must reflect actual backend support. Don't render controls or metrics the runtime doesn't model.

**Required behavior:**

- When Paper artboards (design references) conflict with daemon truth, **daemon wins**.
- Paper governs _composition_; `DESIGN.md` governs _grammar_ (tokens, depth, motion).
- A design reference is **lossy by nature**: demo data, fixture copy, placeholder brand marks, simplified or omitted product content, hand-rolled stand-ins for shipped components, and host chrome redrawn around the named piece are prototype artifacts, never product instructions. Content/data belong to runtime truth, labels/copy to `COPY.md`, marks to the `@compozy/ui` brand inventory, component identity to the `@compozy/ui` inventory + existing domain composites, host chrome to the live surface — record the divergence as an authorized delta (L-032, L-035).
- No invented controls (per-bridge retry/timeout when runtime doesn't support them).
- No invented metrics (no "pending retry" counts when telemetry doesn't expose them).
- Observability-only views are allowed (e.g., Network Peers in v1 has no Disconnect/Remove until backend models them).

**Source:** Multiple plans in `.codex/plans/` (automation-bridges-paper-redesign, network-paper-pages, bridge-web-e2e); L-032 (os-shell menubar mark).

---

## SD-008 — Composition Root Discipline

**Posture.** Only `daemon/` wires components. Reconciliation logic running at boot belongs to composition root and is NOT "legacy support."

**Required behavior:**

- New cross-cutting wiring goes in `internal/daemon/`. Never in subordinate packages.
- Boot reconciliation (`Reconcile`) is composition-root current logic, not legacy.
- Subordinate packages define interfaces and accept implementations via constructors / functional options.
- No back-pointers between subordinate packages.
- The package import graph flows downward only. `internal/daemon` is the only multi-importer.
- `mage Boundaries` is the CI-enforced check; update it in the same commit that introduces a new internal subpackage.

**Source:** Root CLAUDE.md Architecture Principles; `_techspec.md` autonomy boundaries; `.codex/plans/observability-spine.md`, `kb-refac-full-sweep.md`, `remove-legacy-alpha.md`.

---

## SD-009 — Data Exists / Consumer Missing — Build the Consumer

**Posture.** When multiple independent investigations converge on the same data structure as "right shape but unconsumed", the gap is integration ergonomics, not architecture.

**Required behavior:**

- Don't redesign the data when independent slices flag the same data as "right shape, no consumer."
- Build the consumer; preserve the data shape.
- Surface convergence explicitly in research artifacts ("8 of 10 slices flagged the same six lines").
- The autonomy program is the canonical case study: Compozy was 80% built before autonomy started; the work was integration, not invention.

**Source:** `analysis/analysis_global_runs.md` finding 7; `autonomous/analysis/analysis.md`.

---

## SD-010 — Detached Execution Lifetime

**Posture.** Any work that outlives an HTTP/UDS request — prompts, network channel sends, automation jobs — MUST detach via `context.WithoutCancel(ctx)`. Never tie execution lifetime to request lifetime.

**Required behavior:**

- Call long-lived work with `context.WithoutCancel(c.Request.Context())` so client disconnect stops streaming, not execution.
- Expose explicit cancel endpoints (e.g., `POST /api/workspaces/:workspace_id/sessions/:id/prompt/cancel`).
- `context.WithoutCancel` does NOT preserve deadlines — re-attach a deadline if needed.
- The four-cause prompt-stream-stall incident (2026-04-20) is the canonical illustration: HTTP request lifetime tied to prompt → tool_call closed stream → web stop using transport abort → metadata repair classifying `m.pending` as crashed. Each was a separate symptom of the same lifetime-coupling root cause.

**Source:** `.codex/plans/prompt-stream-stall.md`; `_synthesis.md` lesson L1 (4-analysis evidence).

---

## SD-011 — Extensible and Agent-Manageable by Design

**Posture.** Compozy is not only a daemon with UI. It is an extensible runtime that agents must be able to inspect, configure, operate, and repair through structured surfaces. A feature is incomplete if it cannot be extended by Compozy's extension surfaces or managed by agents without relying on the web UI.

**Required behavior:**

- Every PRD, TechSpec, `_tasks.md`, and task body that creates, updates, or removes a feature states the impact on Compozy extensibility surfaces: extensions, hooks, skills/capabilities, tools/resources, registries, bridge SDKs, MCP sidecars, and protocol docs.
- Every user-visible or operator-visible capability has an agent-manageability plan: CLI verbs with structured output, HTTP/UDS parity when daemon state crosses the boundary, deterministic error contracts, discoverable status, and documentation for the agent path.
- Every CLI command, HTTP endpoint, UDS route, generated contract type, and site reference is added, updated, or deleted in the same change as the feature it manages.
- Every `config.toml` addition, update, removal, or no-longer-needed key is handled as a lifecycle change: structs, defaults, merge/overlay behavior, validation, examples, docs, and tests move together.
- "No impact" is allowed only with evidence: the artifact names the checked surfaces and explains why no extension, agent-operation, or config change is needed.

**Source:** explicit user directive on 2026-04-26; reinforces Compozy's product premise (`agent-first`, highly extensible, highly configurable).

**Triggers re-evaluation when:** any spec/feature changes runtime behavior, public contracts, CLI verbs, HTTP/UDS routes, config keys, hooks, extension manifests, skill/tool/resource surfaces, bridge SDKs, or agent-operated workflows.

---

## SD-012 — Sliced Delivery and Proportional Verification

**Posture.** Specs deliver through shippable slices, not layer phases. A slice is the smallest increment that could merge to `main` alone, with an outcome observable from outside the system; its verification is proportional and travels with it. Quality and speed together: the tier covers only what the slice changed, never a full QA cycle per task.

**Required behavior:**

- Task graphs cut by outcome, never by layer: "all backend → all frontend → docs → QA" is an invalid breakdown (L-037). A foundation-only task names the slice that consumes it, and that consumer is next.
- Slice 1 solves the spec's Motivating Problem end-to-end in its simplest honest form; an ADR narrowing or deferring that problem carries the user's recorded sign-off (L-036).
- The slice budget defaults to 5 per spec, overridable per invocation (`slice_budget: N`); an honest overflow is presented as a sequenced program of independently shippable specs and the user decides.
- Every slice carries `## Shippable Outcome` with the cheapest verification tier that can falsify it — `gate` (tests/lints) | `probe` (named CLI/HTTP/UDS command) | `smoke` (real entry path + touched Visual Contract captures). Tier evidence lands with the slice; relocating a slice's gate to a later task is forbidden (L-037). Visual Contract rows derive from the `_uiux.md` inventory, never task self-citation.
- Full QA cycles (journeys, charters, dated reports) remain the trailing QA pair's job — it complements per-slice evidence and never re-walks it (SD-005 unchanged on the tail's existence).
- Per-slice PRs stay opt-in (`--stacked`, off by default); the default single-branch flow keeps one checkpoint commit per slice.

**Source:** agent-comms post-mortem 2026-09-01: 8-feature spec, layer-split graph, all UI in one 7-hour task, gates deferred to a tail task that never named them — first browser verification ~28h after first commit, ~700 review findings (~350 post-SHIP), 9 feature vs 52 repair commits, motivating problem never delivered. Spec set archived at [`compozy-specs/_archived/2026-08-19-agent-comms/`](https://github.com/compozy/compozy-specs/tree/main/_archived/2026-08-19-agent-comms); execution trail on the unmerged `agent-comms` branch (remote only, draft PR [compozy/compozy#497](https://github.com/compozy/compozy/pull/497)). Lessons L-036..L-039.

**Triggers re-evaluation when:** any change to `cy-create-tasks` sizing rules, the QA tail pair, `cy-loop-tasks` phase machine, or a proposed `_tasks.md` whose graph groups work by layer.

---

## SD-013 — Tiered Compatibility: User State Never Breaks

**Posture.** Compozy has had real users since 2026-08-22 and ships on a constant release cadence. The greenfield premise behind SD-002 ("no production deployments to preserve") is false, and v0.3.0 shipped hard cuts that discarded user data (L-040). Compatibility is a contract tiered by who owns the surface — not a binary between "break everything" and "preserve everything". Code quality still wins inside the codebase; users never pay for it.

**Required behavior:**

- **Regime 1 — user state never breaks.** SQLite streams (`compozy.db`, `events.db`, workspace databases), `config.toml`, workspace files, and persisted layouts/profiles always upgrade losslessly. Every shape change ships its migration in the same change, extending the Goose append-only model (L-008, L-021) to every persisted datum. Dropping or truncating user data is allowed only with the user's sign-off recorded in an ADR and a `Migration notes` block in the release note.
- **Regime 2 — public scripted surfaces deprecate before they delete.** CLI verbs/flags and structured output, HTTP/UDS routes and DTOs, hook events, extension/bridge SDK contracts, `config.toml` keys, and `compozy__*` tool IDs. Decision ladder, in order: (a) auto-migrate losslessly at the boundary — the change is free; (b) otherwise keep the old shape working for one release after the new one ships, emit a deprecation warning naming the replacement, delete in the following release; (c) eternal compat is never an option. A surface documented as `experimental` in its docs and CLI help may break without a window; every other surface shipped in a tagged release is `stable`.
- **Regime 3 — internal code stays hard-cut.** Go packages, `web/`, `@compozy/ui`, specs, RFCs, `.compozy/tasks/*`: rename every consumer in one change, delete obsolete code, no aliases, dual fields, or `// legacy` branches. SD-002's internal discipline lives on here.
- **Compat is translation at the boundary.** A shim is a loader/decoder/alias-table entry at the edge (config loader, HTTP/UDS decoder, CLI verb table, migration SQL) — never an `if oldShape` branch in domain code. Only one shim generation exists at a time (N-1, never stacked); each shim's code comment and release note name the release that removes it.
- **Every breaking-change spec lists its delete targets and the regime of each**, with the ladder outcome (auto-migrate / deprecate N→N+1 / `experimental` break) decided before approval. L-006 still governs the enumeration.

**Enforcement (planned, not yet built — do not cite as existing gates):**

- Upgrade-path test lane in release CI: boot the release candidate over the previous release's full `COMPOZY_HOME` fixture; assert zero data loss and zero refused opens.
- Central deprecation registry (shim → replacement → removal release) read by a gate that fails a release still shipping an expired shim, and that generates the release-note "Breaking & migrations" section.
- Stability label rendered in generated CLI, API, and site references.

**Source:** user decision 2026-09-04 confirming the 2026-08-22 proposal; v0.3.0 release notes under `.release-notes/archive/v0.3.0/` — `loop-feedback-semantics-*.md` ("greenfield hard cut that discards existing Loop run history"), `window-tabs-in-the-os-shell-*.md`, `runtime-speed-is-part-of-the-runtime-*.md`, `the-desktop-app-is-now-electron-*.md`, `implement-tasks-replaces-software-delivery-*.md`. Lesson L-040.

**Triggers re-evaluation when:** any migration drops or truncates rows; any PR removes or renames a CLI verb/flag, HTTP/UDS route, DTO field, hook event, config key, or tool ID without a deprecation window; any shim lacks a removal release; any `// legacy` or `if oldShape` branch appears inside domain code.
