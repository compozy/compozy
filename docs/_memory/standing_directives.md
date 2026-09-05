# Standing Directives

Current engineering decisions, scoped to the surfaces they govern. Read matching directives when relevant; historical examples and retired entries do not create additional workflow stages.

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
- When the user corrects a premise or reports dissatisfaction, revisit the affected decision and explain the adjustment. Isolated words do not automatically require another question.

**Source:** `.codex/ledger/` notes; recurring pattern across all sessions.

---

## SD-004 — Model Selection and Delegation

Use the model configured for the current session unless the user selects another or the task requires a demonstrated capability difference. Historical model/version assignments are not defaults. Pin a model only when reproducibility or the user's request needs it.

Delegate bounded independent work when authorized and useful. Supply the outcome, scope, relevant evidence, and ownership; reuse existing research. Subagents are read-only unless their assignment explicitly grants writes, and they preserve others' changes. The parent accepts concrete artifacts, not a worker's unsupported success claim.

**Source:** historical multi-LLM workflow evidence; revalidated for the user's instruction-simplification request on 2026-09-04.

---

## SD-005 — Real-Scenario QA at the Relevant Scope

Release/scenario validation and changed user-visible, provider, persistence, or cross-surface contracts need real evidence from their affected journeys. Pure editorial work and internal changes already covered by an owning deterministic check do not create a full QA lab.

A requested full spec-cycle program retains its trailing `qa-report`/`qa-execution` phase contract; that phase covers remaining integration journeys once and reuses current slice evidence. Ordinary tasks verify the changed scenarios without adopting the whole program.

QA state lives in `docs/qa/`: content-addressed scenario/bug files, journeys, charters, and dated reports; `state.csv` is a generated view. Labs use `eng-qa-bootstrap`, isolate concurrent runtime homes/ports/sockets, respect provider auth/home policy, and tear down processes on every terminal path (L-029).

**Source:** autonomy/Hermes QA incidents and current QA skills; scoped by the 2026-09-04 simplification decision.

---

## SD-006 — Evidence-Led Bug Fixes

Use the narrowest reproduction or already-collected incident evidence to identify the cause. If reproduction is unavailable, record the observed evidence and uncertainty; a timestamp template is not a prerequisite to investigation.

Repair the owning production cause, preserve valid assertions, and verify the original symptom plus affected contracts. Add a regression only when the existing suite/gate does not cover that invariant. Repeat or broaden checks after relevant changes, failures, or unresolved risk.

**Source:** prompt/session/terminal incident plans and QA findings; L-001, L-007, and L-039.

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

## SD-009 — Integrate Existing Data Before Redesigning It

When evidence shows an adequate existing data model lacks a consumer, implement the smallest needed consumer instead of redesigning the model without contrary evidence. Record the conclusion and its source; fixed investigation counts or a separate convergence report are unnecessary.

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

Runtime capabilities must be extensible and operable by agents through structured surfaces. UI-only manageability is incomplete.

For a feature or contract change, record the affected extension/hook/resource/registry/bridge/MCP surfaces, CLI/HTTP/UDS operations and errors, config lifecycle, workspace isolation, and Web/Docs impact once at the owning artifact (`change-impact.md`). Dependent tasks link to that analysis and update only their deltas.

Co-ship affected contracts, defaults/overlays/validation, code, generated references, docs, and owning tests. Config and public-surface changes follow SD-013. An unaffected entry names the checked boundary and why the change does not touch it; editorial/internal work does not need a full repeated matrix.

**Source:** user directive 2026-04-26; Compozy's extensible, agent-manageable product premise.

---

## SD-012 — Sliced Delivery and Proportional Verification

Deliver observable outcomes with verification proportional to each change. Choose slice count from dependencies and risk; a configured budget is a planning aid, not a reason to omit the motivating problem or force another approval round. A foundation task names its consuming outcome and verifies its own boundary.

The spec identifies the earliest useful end-to-end outcome. Narrowing an accepted goal requires the user's recorded decision (L-036). Each slice carries its applicable `gate`, `probe`, or `smoke` evidence; named-reference captures come from the actual UI contract. Reuse current evidence instead of repeating it at every checkpoint.

When the full spec-cycle workflow is requested, retain its state/phase schema and trailing integration QA ownership. Separate PRs/review rounds are chosen by that workflow and user scope; ordinary fixes do not acquire the whole pipeline.

**Source:** agent-comms post-mortem 2026-09-01; L-036..L-039. Historical evidence and counts remain in those lessons.

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
