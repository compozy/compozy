# Spec Authoring Playbook

Mandatory preflight reading for `$cy-create-prd`, `$cy-create-techspec`, and `$cy-create-tasks`. Distilled from 8 forensic analyses of past PRDs, TechSpecs, task trees, and Codex sessions. Every directive carries an evidence reference — open the linked lesson or analysis to see the incident behind the rule.

> **Loading contract**: agents authoring an `_idea.md`, `_prd.md`, `_techspec.md`, `_tasks.md`, or `task_NN.md` MUST read this playbook before producing the artifact. The `cy-spec-preflight` skill enforces this.

---

## 1. Authoring Posture (cross-cutting)

Applies to PRD + TechSpec + Tasks alike. See `docs/_memory/standing_directives.md` for full text.

- **Greenfield-alpha = name delete targets.** Every breaking-change spec lists what disappears. No compat shims, no aliases, no schema fallbacks. → `lessons/L-006`, SD-002.
- **Hard-cut renames.** Renames sweep code, storage, APIs, CLI, extensions, specs, RFCs, AND `.compozy/tasks/*` artifacts in the same change. → `analysis/analysis_codex_plans.md` Theme 4.
- **Extensible and agent-manageable by design.** Every create/update/remove feature decision includes extension-surface impact, agent-operable CLI/HTTP/UDS support, and `config.toml` lifecycle impact. → SD-011.
- **Final decisions on the spec.** No "TBD" / "we'll decide later" — every fork is resolved before approval. → codex_sessions §Engineering Principle 3.
- **Cite competitor refs.** Every spec that draws on `.resources/<repo>` lists the file paths so the implementer reads them too. → codex_sessions §Engineering Principle 4.
- **BR-PT in conversation, EN in artifacts.** → SD-003.
- **Subagents default to read-only.** Use them to research and return findings; the parent agent authors files. A subagent may write/edit only when the parent's prompt explicitly delegates that action. Skills with hard read-only contracts (`cy-spec-peer-review`, `deep-review`) override this default. → user-memory `feedback_subagents_readonly.md`.
- **Two-touch rule.** Third change to the same area opens a new TechSpec, not a third patch. → user-memory `feedback_two_touch_rule.md`.
- **Multi-LLM pipeline.** Codex (`gpt-5.4` with `reasoning_effort=xhigh`) authors; Claude Opus pressure-tests; `gpt-5.4-mini` with `reasoning_effort=high` explores breadth when explicitly delegated. → SD-004.

---

## 2. Phase: PRD (`$cy-create-prd`)

PRDs frame **what** and **why**. They do not frame **how**.

### MUST contain

- Problem statement, user/operator impact, current pain.
- Agent/operator manageability outcome: who or what must inspect, configure, operate, or repair the capability outside the web UI.
- Extension ecosystem expectation: whether third-party/runtime extension points should participate, without naming implementation details.
- Goals + non-goals listed explicitly (non-goals are not inferred — they're stated).
- Success criteria observable from outside the system.
- Open Questions capturing unresolved product choices without inventing answers.
- Architecture Decision Records section linking any PRD decision ADRs.

### MUST NOT contain

- Framework names, storage engines, file formats, transport choices, error codes, schema details. Strip and push to TechSpec. → `lessons/L-013`.
- Open questions about decisions already constrained by accepted ADRs or product non-goals. Carry decided constraints forward; ask for single-option confirmation only. → codex_sessions §Engineering Principle 6.
- Implementation milestones masquerading as user goals.

### Validate before exit

- Run the `cy-spec-preflight` PRD check and the implementation-leak script.
- Get user approval on the complete draft before saving.

---

## 3. Phase: TechSpec (`$cy-create-techspec`)

The autonomy `_techspec.md` is the high-water mark. Six markers correlate with **clean execution** (one review round) vs. heavy rework. → `analysis/analysis_compozy_tasks.md` §PRD/TechSpec Quality Patterns.

### Six quality markers (ALL six required)

1. **MVP Boundary statement** at the top. Names which numbered tasks compose MVP, what is post-MVP, what is explicitly out of scope.
2. **Architectural Boundaries** section. Enumerates which packages can/cannot import which. Names new internal packages explicitly. References `daemon/` composition root.
3. **Concrete Go interface signatures** pasted as code blocks (not described in prose). Every method signature final.
4. **Data-model field rationale**. Any new SQLite columns, frontmatter fields, or config keys are listed with purpose + shape. JSON metadata blobs forbidden when a column or side-table fits.
5. **Side-table-vs-JSON decision** stated for every new domain entity. Side-tables for matchable state; JSON for opaque metadata only.
6. **Lease/safety invariants enumerated as a numbered list**, not prose. Concurrency- or ownership-sensitive code paths.

→ `lessons/L-012`.

### MUST also contain

- **Forensic frame** when the spec attacks a real incident: open with confirmed reproduction (timestamp, command, observed evidence). → SD-006, `analysis/analysis_codex_plans.md` Pattern 1.
- **"No fallback / no compat shim / no placeholder" clauses** that pre-empt drift. → `analysis/analysis_codex_plans.md` Pattern 2.
- **Phased plan**: safe cleanup phases separated from behavior-changing edits. Each phase has its own verification gate. → Pattern 3.
- **Test plan = per-section bullet list** with concrete assertions and verification commands (`make verify`, `make web-test`, etc.). Not "tests will pass". → Pattern 4.
- **Public Interfaces / Types** section enumerating routes/payloads/CLI verbs/config keys added/changed. → Pattern 5.
- **Extensibility Integration Plan** section enumerating extension manifests, hooks, skills/capabilities, tools/resources, bundles, registries, bridge SDKs, MCP sidecars, and protocol docs that are added/changed/removed or explicitly unaffected. → SD-011.
- **Agent Manageability Plan** section enumerating CLI verbs, HTTP endpoints, UDS routes, structured outputs, status/config discovery, and deterministic errors agents will use to operate the feature. UI-only control is incomplete. → SD-011.
- **Config Lifecycle** section enumerating `config.toml` keys/defaults, merge/overlay behavior, validation, examples, generated CLI/site docs, and tests that are added/changed/removed or explicitly unaffected. → SD-011.
- **Assumptions/Defaults** section closing the spec, pre-empting "what if" questions. → Pattern 6.
- **Web/Docs Impact** for any contract change (`internal/api/contract`, OpenAPI, CLI verb). Activate `cy-web-docs-impact`.

### MUST refuse

- Generic event bus, reflection-based routing, or remote-carrier infrastructure without a shipped consumer. → CLAUDE.md Architecture, `lessons/L-005`.
- Parallel queue alongside `task_runs`. Add columns + side-tables instead. → `lessons/L-003`.
- Hooks that tail event tables. Hooks dispatch at the call site. → CLAUDE.md.
- Duplicate ownership state in JSON metadata. → `lessons/L-003`.
- Compat shims, `@deprecated` stubs, dual-naming, schema fallback paths. → `lessons/L-006`, SD-002.
- Manual paths and autonomous paths as separate code. They converge on same primitives. → `lessons/L-004`.
- Authoritative state transitions replicated across peer packages. → `lessons/L-005`.
- UI-only manageability for an agent-operated capability. Agents need structured CLI/HTTP/UDS paths. → SD-011.
- Raw `claim_token` (or any secret) crossing transport/log/UI/memory. Use hash forms. → CLAUDE.md Security Invariants.
- `EnsureSchema` for column changes. Numbered migrations only. → `lessons/L-008`.
- Config keys added/renamed/removed without same-change structs, defaults, merge/overlay, validation, examples, docs, and tests. → SD-011.
- Tying execution lifetime to request lifetime. Detached work uses `context.WithoutCancel`. → `lessons/L-001`.

### Validate before exit

- All six markers present (cy-spec-preflight checklist).
- After the user approves the baseline draft and it is saved, offer peer review via `cy-spec-peer-review` (Opus xhigh). Run it only if the user opts in, summarize the findings, let the user choose what to incorporate, and ask whether to run another round or stop.

---

## 4. Phase: Tasks (`$cy-create-tasks`)

Tasks turn a TechSpec into an implementable dependency graph. The structure of `_tasks.md` is load-bearing.

### `_tasks.md` shape

- Table columns: `# | Title | Status | Complexity | Dependencies`. Preserve column order across edits.
- **MVP Boundary** section above the table: name which tasks implement the MVP, which trailing tasks cover QA planning/execution, and what remains post-MVP.
- **Final two rows** are always `qa-report` (high) + `qa-execution` (critical), operating on the living `docs/qa/` tree (state.csv, journeys, charters, bug registry, dated reports). UI-bearing features include e2e in qa-execution. → `cy-tasks-tail-qa-pair` skill.

### Per-row directives

- **Dependencies** column is first-class. Every task lists `task_NN` predecessors or `-`. The dependency graph IS the execution order.
- **Complexity** rated explicitly: `low | medium | high | critical`. Critical reserved for safety primitives (lease, claim, scheduler, coordinator) and the final QA execution.
- **Skills** are named in each task body when explicit activation is required. Multi-domain tasks list all relevant skills in the body.
- **Web/Docs Impact** subsection is mandatory for backend task bodies, even when "none". Activate `cy-web-docs-impact` to populate. → `cy-web-docs-impact` skill.
- **Extensibility / Agent Manageability / Config Lifecycle** subsections are mandatory for feature-bearing backend tasks, even when the answer is explicit "none with evidence". → SD-011.

### MUST contain (per task)

- **`.resources/<competitor>` references** when the TechSpec drew on competitors. Each task body lists exact file paths so the implementer reads them. → codex_sessions §Engineering Principle 4.
- **Agent-operability tests** when a task adds/changes CLI/HTTP/UDS behavior: compare structured CLI output with HTTP/UDS state where applicable and assert deterministic errors. → SD-011.
- **Test density that is NOT "fraco"**. Plan unit + integration + e2e proportional to behaviors documented in the TechSpec. Reject lists with 1-2 tests for many behaviors. → `lessons/L-011`.
- **Reconciled status**. If `task_03` is `pending` while `task_10` is `completed`, fix the table — that's drift. → `analysis/analysis_compozy_tasks.md` §unified-capabilities drift.
- **No TBD / placeholder rows**. → SD-006, codex_sessions §Engineering Principle 3.

### Validate before exit

- Run `cy-tasks-tail-qa-pair` to ensure the QA pair closes the list.
- Run `cy-web-docs-impact` to populate the impact subitems.
- Confirm dependency graph has no cycles.

---

## 5. Phase: Per-Task Body (`task_NN.md`)

Each task file is the unit of execution. The autonomy task files are the canonical pattern.

### Header

- `<critical>ALWAYS READ _techspec.md, every per-task ADR file if present, and every per-task memory file before editing.</critical>` block at the top.
- `<critical>MINIMIZE CODE, TESTS REQUIRED, NO WORKAROUNDS</critical>` block.
- Reference to the `_techspec.md` section being implemented.

### Body sections

- **Goal**: 1-3 sentences naming the deliverable.
- **Files / Surfaces**: enumerated list of files/packages touched.
- **Implementation Steps**: numbered, deterministic.
- **Tests**: enumerated assertions covering happy path + failure paths + concurrency stress where relevant. → `agh-test-conventions` skill, `lessons/L-002`, `agh-cleanup-failure-paths` skill.
- **Web/Docs Impact**: paths affected under `web/` and `packages/site`, OR explicit `none — backend-only` line. → `cy-web-docs-impact` skill. Includes the **QA impact** line: `docs/qa/state.csv` scenario ids to reset to `untested` when user-visible behavior changes (flag, don't retest), OR `none — no user-visible behavior change`.
- **Extensibility / Agent Manageability / Config Lifecycle**: affected extension hooks/manifests/skills/tools/resources/bundles/registries/bridge SDKs, CLI/HTTP/UDS agent operation paths, and `config.toml` keys/docs/tests, OR explicit `none — checked surfaces: ...` line.
- **References**: `.resources/<competitor>/path` paths cited from the TechSpec.
- **Completion Notes** (filled at execution time): commands run, coverage %, "not changed" disclaimers for `web/`/`packages/site`.

### Memory template

Workflow memory under `.compozy/tasks/<slug>/memory/task_NN.md` follows the 6-section template: **Objective Snapshot / Important Decisions / Learnings / Files & Surfaces / Errors & Corrections / Ready for Next Run**. → `analysis/analysis_compozy_tasks.md` §Memory File Findings, `cy-workflow-memory` skill.

### MUST refuse

- Tests with `_ = errFn()` discards.
- Status-code-only assertions without body/contract evidence.
- `t.Parallel()` on env-mutating tests. → `lessons/L-002`.
- `force: true` on Playwright actionability checks.
- `http.DefaultClient` in production code paths. → `agh-cleanup-failure-paths`.
- Schema changes without numbered migration. → `lessons/L-008`.

---

## 6. Anti-Patterns Refused (negative list)

If the agent's draft contains any of these, refuse to mark the artifact ready:

- **Generic event bus / reflection routing / speculative remote-carrier infrastructure.**
- **Parallel queue alongside `task_runs`.**
- **Hooks tailing event tables** (instead of dispatching at call sites).
- **`@deprecated` stubs, dual-naming, "preserve old" branches.**
- **Compat shims** for unreleased alpha state.
- **TUI quit auto-stops session** (already reverted; do not re-introduce).
- **Boolean-prop proliferation** in UI components (favor many small components).
- **Cron / schedule CI workflows** (heavy tests live in release PR `dry-run`). → user-memory `feedback_ci_no_cron.md`.
- **PRD naming frameworks/storage/error codes/file formats.** → `lessons/L-013`.
- **TechSpec prose-only without Go interface signatures.** → `lessons/L-012`.
- **`_tasks.md` test plan that is "fraco" / "leviano" / 1-2 tests for many behaviors.** → `lessons/L-011`.
- **`EnsureSchema`-style boot reconciliation for column changes.** → `lessons/L-008`.
- **Tying long-lived execution to request context.** → `lessons/L-001`.
- **Splitting manual operator paths from autonomous paths.** → `lessons/L-004`.
- **Letting peer packages claim runs.** Only `task.Service.ClaimNextRun` claims. → `lessons/L-005`.
- **Raw `claim_token` over the wire / in logs / in memory.** → CLAUDE.md Security Invariants.
- **Agent-operated features that only expose web UI controls.** → SD-011.
- **Config changes without same-change docs, examples, validation, and tests.** → SD-011.

---

## 7. Cross-References

| Topic                                       | Where to read                                                          |
| ------------------------------------------- | ---------------------------------------------------------------------- |
| Standing posture                            | `docs/_memory/standing_directives.md` (SD-001..SD-011)                 |
| Lessons (incident → rule)                   | `docs/_memory/lessons/README.md` (L-001..L-013)                        |
| Vocabulary                                  | `docs/_memory/glossary.md`                                             |
| Cross-source synthesis                      | `docs/_memory/_synthesis.md` and `docs/_memory/analysis/analysis_*.md` |
| Spec quality evidence                       | `docs/_memory/lessons/L-012-techspec-prose-only-rework.md`             |
| Skill: peer review                          | `.agents/skills/cy-spec-peer-review/`                                  |
| Skill: tasks tail QA pair                   | `.agents/skills/cy-tasks-tail-qa-pair/`                                |
| Skill: web/docs impact                      | `.agents/skills/cy-web-docs-impact/`                                   |
| Skill: test conventions                     | `.agents/skills/agh/agh-test-conventions/`                             |
| Skill: cleanup failure paths                | `.agents/skills/agh/agh-cleanup-failure-paths/`                        |
| Skill: schema migration                     | `.agents/skills/agh/agh-schema-migration/`                             |
| Skill: contract codegen co-ship             | `.agents/skills/agh/agh-contract-codegen-coship/`                      |
| Skill: worktree isolation                   | `.agents/skills/agh/agh-worktree-isolation/`                           |
| Skill: spec preflight (loads this playbook) | `.agents/skills/cy-spec-preflight/`                                    |
| Root rules                                  | `/CLAUDE.md`, `/AGENTS.md`                                             |
| Web rules                                   | `/web/CLAUDE.md`                                                       |
| Site rules                                  | `/packages/site/CLAUDE.md`                                             |
