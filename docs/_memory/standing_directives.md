# Standing Directives

Ongoing engineering posture, not date-stamped per-task plans. These are perpetually active rules. Surfaced from undated entries in `.codex/plans/` and verified across the synthesis corpus.

---

## SD-001 — Long-Running Sessions Supervision

**Posture.** AGH sessions can run for hours. Supervise activity, don't wait on wall-clock timeouts. Heartbeats, progress events, and idempotent cancel are the supervision primitives.

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

## SD-002 — Remove Legacy Alpha Compatibility Code

**Posture.** AGH is greenfield alpha with zero production users. Compat shims, nil-receiver legacy stubs, legacy-meta no-ops, and dual code paths for "old behavior" are forbidden. **Delete the old thing.** This is a stronger, perpetual application of the CLAUDE.md "Greenfield Alpha — Zero Legacy Tolerance" rule.

**Required behavior:**

- Strip nil-receiver fallbacks, `legacySessionMeta` mappers, no-op compat methods, and "preserve old field name" branches.
- Renames are **hard cuts**: code, storage, APIs, CLI, extensions, specs, RFCs, and `.compozy/tasks/*` artifacts in the same change. No aliases, no dual fields.
- Distinguish "still useful current logic" (`Reconcile` at composition root) from "legacy support" (delete). Composition-root reconciliation is NOT legacy.
- One-pass legacy repair is allowed only in the narrow case where the cost of "delete the old thing" is "every developer rebuilds their local SQLite". Document the boundary in an ADR (e.g., `session-driver-override/adrs/adr-005.md`) and switch to strict semantics immediately after repair.

**Source:** `.codex/plans/remove-legacy-alpha.md` (undated standing directive). Multiple ADRs reinforce.

**Triggers re-evaluation when:** any PR proposing schema-version branching, `// legacy` markers, dual-naming, or "preserve for compat" logic.

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

**Posture.** AGH development uses three LLMs with deliberate role assignment.

**Required behavior:**

- **Codex (`gpt-5.4` with `reasoning_effort=xhigh`)** authors TechSpecs, major Go code, autonomous-mode kernel work.
- **Claude Opus (`xhigh`)** pressure-tests TechSpecs in user-directed cross-LLM review rounds, reviews architecture decisions, writes/reviews React/E2E frontend code.
- **`gpt-5.4-mini` with `reasoning_effort=high`** runs as parallel subagents for breadth (codebase mapping, competitor analysis, conversation-log auditing) when explicitly delegated.
- Do not substitute models without explicit user approval.
- Subagents default to read-only — they return analysis to the parent agent, and the parent writes any required files. They may write/edit/commit only when the parent's prompt explicitly delegates that action; otherwise the parent authors the change. Skills with stricter contracts (e.g. `cy-spec-peer-review`, `deep-review`) keep their hard read-only rule for their dispatch lane.

**Source:** Direct quotes across many sessions; codified in `feedback_multi_llm_pipeline.md` (user memory).

---

## SD-005 — Real-Scenario QA Before Release

**Posture.** `make verify` is necessary but not sufficient. Real-scenario QA against a multi-agent / multi-channel / multi-task workspace catches drift `make verify` misses.

**Required behavior:**

- Every program ends with a `qa-report` task and a `qa-execution` task. UI-bearing features include browser-based e2e.
- QA state is living repo docs: the pair operates on the committed `docs/qa/` tree (`state.csv` scenario tracker, global `bugs/BUG-NNNN.md` registry, journeys, session charters, dated `reports/<date>-<scope>.md`). Per-round `qa/` trees and reset bug ids are the retired anti-pattern.
- For release validation on the multi-agent runtime, the `real-scenario-qa` skill runs a playbook lab (bootstrap via `agh-qa-bootstrap`, one in-persona operator kickoff, runtime observation, strict audit) and delegates planning/execution mechanics to the `qa-report` + `qa-execution` pair; its findings also land in `docs/qa/`.
- Hermetic QA still respects each provider's auth contract: bound-secret and brokered lanes isolate `PROVIDER_HOME`, while `native_cli` providers with `home_policy=operator` keep the operator `HOME` / native login state unless the scenario explicitly validates isolated provider-home behavior.
- Concrete bug evidence (autonomy task_18 BUG-001/002/003, Hermes BUG-001..007) shows the QA pass surfaces real production bugs the unit/lint/build coverage cannot catch.

**Source:** Codex sessions (most-repeated request); `qa-report`/`qa-execution` SKILL.md (living-docs contract); `real-scenario-qa` SKILL.md (runtime-observation harness); autonomy and Hermes QA verification reports (historical).

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

**Source:** `.codex/plans/` (consistent forensic frame in `child-workgroup-activation.md`, `session-stop-hang.md`, `dashboard-xterm-visibility.md`, `prompt-stream-stall.md`); also encoded in `qa-execution` (fix-loop governor: regression test red-before/green-after), `real-scenario-qa`, and `cy-fix-reviews`.

---

## SD-007 — Truthful UI > Plausible UI

**Posture.** UI must reflect actual backend support. Don't render controls or metrics the runtime doesn't model.

**Required behavior:**

- When Paper artboards (design references) conflict with daemon truth, **daemon wins**.
- Paper governs _composition_; `DESIGN.md` governs _grammar_ (tokens, depth, motion).
- A design reference is **lossy by nature**: demo data, fixture copy, placeholder brand marks, and simplified or omitted product content are prototype artifacts, never product instructions. Content/data belong to runtime truth, labels/copy to `COPY.md`, marks to the `@agh/ui` brand inventory — record the divergence as an authorized delta (L-032).
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
- The autonomy program is the canonical case study: AGH was 80% built before autonomy started; the work was integration, not invention.

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

**Posture.** AGH is not only a daemon with UI. It is an extensible runtime that agents must be able to inspect, configure, operate, and repair through structured surfaces. A feature is incomplete if it cannot be extended by AGH's extension surfaces or managed by agents without relying on the web UI.

**Required behavior:**

- Every PRD, TechSpec, `_tasks.md`, and task body that creates, updates, or removes a feature states the impact on AGH extensibility surfaces: extensions, hooks, skills/capabilities, tools/resources, bundles, registries, bridge SDKs, MCP sidecars, and protocol docs.
- Every user-visible or operator-visible capability has an agent-manageability plan: CLI verbs with structured output, HTTP/UDS parity when daemon state crosses the boundary, deterministic error contracts, discoverable status, and documentation for the agent path.
- Every CLI command, HTTP endpoint, UDS route, generated contract type, and site reference is added, updated, or deleted in the same change as the feature it manages.
- Every `config.toml` addition, update, removal, or no-longer-needed key is handled as a lifecycle change: structs, defaults, merge/overlay behavior, validation, examples, docs, and tests move together.
- "No impact" is allowed only with evidence: the artifact names the checked surfaces and explains why no extension, agent-operation, or config change is needed.

**Source:** explicit user directive on 2026-04-26; reinforces AGH's product premise (`agent-first`, highly extensible, highly configurable).

**Triggers re-evaluation when:** any spec/feature changes runtime behavior, public contracts, CLI verbs, HTTP/UDS routes, config keys, hooks, extension manifests, skill/tool/resource surfaces, bridge SDKs, or agent-operated workflows.
