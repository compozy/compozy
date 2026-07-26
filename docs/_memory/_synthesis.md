# Synthesis: Findings Across 8 Analyses

Cross-referenced output from `analysis/analysis_codex_sessions.md`, `analysis/analysis_global_runs.md`, `analysis/analysis_local_runs.md`, `analysis/analysis_compozy_tasks.md`, `analysis/analysis_codex_ledger.md`, `analysis/analysis_codex_plans.md`, `analysis/analysis_qmd_collections.md`, and `analysis/analysis_existing_surfaces.md`. Each finding is tagged with the sources that support it (count of analyses and a short citation). **Higher source count = stronger signal.**

This document is a _review surface_ for Pedro to approve/reject before anything is generated as a real skill, lesson, or CLAUDE.md edit.

---

## Top-Level Findings

1. **Test discipline is the #1 source of review noise.** ~40% of all CodeRabbit issues across `autonomous`, `hermes`, `qa-review`, `unified-capabilities`, `session-driver-override` are the same complaint: missing `t.Run("Should...")`, missing `t.Parallel()` (with `t.Setenv` correctly rejected), `_ = json.Marshal`, status-code-only assertions. Reviewers literally quote Pedro's own CLAUDE.md back at him. (5 analyses)

2. **Pedro runs a deliberate multi-LLM dev pipeline that CLAUDE.md doesn't capture.** Codex (gpt-5.4-xhigh) authors specs → Claude Opus reviews them → gpt-5.4-mini-high explores. Subagents are read-only. The patterns are stated almost verbatim across many sessions. (codex_sessions, codex_ledger)

3. **Five rules are repeated in every backend session and not in CLAUDE.md**: (a) auto-append `$qa-report` + `$qa-execution` to every `_tasks.md`; (b) every backend task has a Web/Docs Impact subitem; (c) parallel agents need unique `AGH_HOME` + ports; (d) cross-LLM techspec peer review before approval; (e) cite `.resources/<competitor>` paths in tasks. (codex_sessions: most-repeated requests)

4. **The autonomy ADRs (002–012) and `_techspec.md` encode load-bearing rules that aren't in CLAUDE.md.** Manual-first contract (ADR-010), claim/lease invariants (ADR-003), coordinator triggers (ADR-005), safe spawn (ADR-006), MVP message kinds (ADR-007), hook taxonomy (ADR-009), generated-contracts-co-ship (ADR-011), coordination channels (ADR-012). (existing_surfaces, compozy_tasks)

5. **CLAUDE.md is materially stale on package layout and build commands.** Missing: `internal/scheduler`, `internal/agentidentity`, `internal/situation`, `internal/hooks`, `internal/task`, `internal/network`, `internal/resources`, `packages/site`. Missing build commands: `make codegen`, `make codegen-check`, `make test-e2e-web`, `make test-e2e-nightly`, `make test-integration`. Phase ordering is outdated. (existing_surfaces)

6. **Three new skills exist locally and aren't in dispatch:** `.agents/skills/agh/real-scenario-qa/` (project-local). The pattern was distilled from autonomy `task_18` QA. Not yet wired to the CLAUDE.md skill dispatch table. (existing_surfaces, codex_sessions)

7. **The greenfield-alpha discipline is real and visible** in the corpus. Renames are hard-cut (`network-rename`, `assistant-ui`, `workspace-menu`), schema migrations are direct rewrites, no compat shims. Reviews flag "preserve old behavior" PRs. (codex_plans, codex_ledger, multiple)

---

## Skill Candidates

Listed in 3 priority bands. Each entry: name → trigger → mandate → evidence sources.

### HIGH priority (multi-source evidence, immediate value)

#### S-H1. `cy-tasks-tail-qa-pair`

- **Trigger**: after `cy-create-tasks` finishes generating `_tasks.md`.
- **Mandate**: append two trailing tasks following the `.compozy/tasks/hermes` template — a `$qa-report` task and a `$qa-execution` task. UI-bearing features include e2e (Playwright or browser-use) in the qa-execution task.
- **Evidence**: codex_sessions (most-repeated request, near-verbatim across 6+ sessions); compozy_tasks (every active program ends with a QA pair: autonomy task_17/18, hermes task_10/11). Pedro corrected Codex multiple times with this exact ask.

#### S-H2. `cy-spec-peer-review`

- **Trigger**: a TechSpec is drafted and ready for approval.
- **Mandate**: invoke `compozy exec --ide claude --model opus --reasoning-effort xhigh --format json --prompt-file <prompt>`; capture findings; return blockers, nits, readiness. Resolve blockers before approval.
- **Evidence**: codex_sessions (every major techspec); codex_ledger ("Opus rounds 1, 2, web/site impact" on autonomous techspec). Pedro never approves a major techspec without this.

#### S-H3. `cy-web-docs-impact`

- **Trigger**: any backend task draft or implementation.
- **Mandate**: produce a "Web/Docs Impact" subitem listing affected `web/` routes/components/hooks AND affected `packages/site` doc pages. Backend-only tasks may declare "no impact" but only after analysis.
- **Evidence**: codex_sessions (literally every backend session — Pedro asks "não é preciso mudar nada na UI do web/?" almost every time); compozy_tasks (autonomy `_techspec.md` step boundaries spell this out per task).

#### S-H4. `agh-test-conventions` (or extend `testing-boss`)

- **Trigger**: any time a Go test file (`*_test.go`) is being written or modified.
- **Mandate**: enforce (a) every case in `t.Run("Should ...")` subtest; (b) `t.Parallel()` default, opt-out only with comment for `t.Setenv` or shared state; (c) no `_ = errFn(...)` in tests; (d) status-code-only assertions also assert body or error message; (e) deterministic time/IDs; (f) compile-time interface assertions for new types.
- **Evidence**: ~40% of ALL review issues across all PRs are this category (compozy_tasks counted ~29 issues in autonomy round 1 alone; global_runs documents 12+ separate quotes; codex_ledger; local_runs; codex_sessions). Reviewers quote CLAUDE.md verbatim and still find violations.

#### S-H5. `agh-cleanup-failure-paths`

- **Trigger**: editing a function with multi-step setup/teardown, subprocess spawn, registry registration, or context creation.
- **Mandate**: enumerate every error-return; require explicit `cancel()`, `Close()`, `Stop()`, lease-release, or process-stop on each. Forbid `http.DefaultClient` for outbound calls. Test fail-paths.
- **Evidence**: hermes-001 issue_001 (procCtx leak); hermes-001 issue_015 (`http.DefaultClient` no timeout); hermes round-2 issue_010 (logout silently fails on remote revoke); autonomy expired-lease cleanup issues. (compozy_tasks, global_runs, local_runs)

#### S-H6. `agh-schema-migration`

- **Trigger**: any change to a SQLite column/index/constraint, any new struct field that round-trips through SQLite.
- **Mandate**: confirm a numbered migration entry exists; reject `EnsureSchema`-style boot reconcile for column additions; test fresh-DB AND reopen-after-restart paths; record migration in `schema_migrations`.
- **Evidence**: hermes-001 issue_020 was Critical (memory_operation_log widened without migration). Repeated across hermes/autonomy. Hermes Track 1 was rewritten partly to enforce a single migration primitive. (compozy_tasks, global_runs, codex_ledger, local_runs — 4 analyses)

#### S-H7. `agh-contract-codegen-coship`

- **Trigger**: edits in `internal/api/contract/**`, `internal/api/spec/**`, `web/src/generated/**`, or `openapi/**`.
- **Mandate**: regenerate `openapi/agh.json` and `web/src/generated/agh-openapi.d.ts` in same PR; update `web/src/systems/*/types.ts` consumers + Storybook/MSW fixtures; pass `make codegen-check`, `make web-typecheck`, `make web-test`.
- **Evidence**: ADR-011 (autonomous) explicitly mandates this; "co-ship" mentioned in compozy_tasks, codex_plans, codex_ledger. Hermes Task 5's first verify failed because settings MCP fixtures didn't carry `transport`. (4 analyses)

#### S-H8. `agh-worktree-isolation`

- **Trigger**: a QA execution or test run is being prepared while user signals (env or task arg) that other agents run in parallel worktrees.
- **Mandate**: enforce per-worktree unique `AGH_HOME`, unique daemon ports, unique `tmux-bridge` socket paths. Block ops that would write to `~/.agh/` or default ports.
- **Evidence**: codex_sessions (asked near-verbatim across multiple sessions); local_runs (`_worktrees/harness/.compozy/runs/...` confirms parallel execution); local_runs ("concurrent worktree commits will deadlock" — `.git/index.lock` contention).

### MEDIUM priority (clear evidence, narrower scope)

#### S-M1. `agh-acp-driver-lifecycle`

- **Mandate**: codify the 4-step activation (`driver.Start` → `attachProcess` → `MarkAgentReady` → publish `ReadySubject`); ACP wrapper process-group launch/kill on Unix; cooperative cancel-then-grace stop semantics; Windows forced-exit fallback.
- **Evidence**: codex_plans (`child-workgroup-activation.md`, `session-stop-hang.md`, `long-running-sessions.md`, `prompt-stream-stall.md`, `daemon-runtime-dashboard.md`); local_runs; codex_ledger.

#### S-M2. `agh-cli-flag-discipline`

- **Mandate**: trim string-list inputs and drop empty/whitespace; use `cmd.Flags().Changed(name)` (Cobra) to distinguish "not set" from "zero value"; never silently ignore explicit overrides.
- **Evidence**: autonomous-001 issue_025 (`--kind` silently ignored); autonomous-001 issue_030 (whitespace `--capability` survived). (compozy_tasks, global_runs)

#### S-M3. `agh-secret-redaction-audit`

- **Mandate**: claim*token (`agh_claim*\*`), MCP auth tokens, OAuth codes, PKCE verifiers, secret bindings never appear in logs/status/settings/error payloads/SSE/web/memory. Use hash forms (`claim_token_hash`).
- **Evidence**: codex_ledger (Hermes Task 5 + autonomy Task 9 codified the patterns); compozy_tasks (ADR-003/011/012). Autonomy domain validation rejects raw `claim_token` in result metadata.

#### S-M4. `agh-symlink-escape-hardening`

- **Mandate**: skill sidecars, skill files, managed-extension dependency copies, bundle install paths verify resolved targets remain inside approved roots. Use `EvalSymlinks` + path-prefix check, not naive joins. Handle macOS `/private/var/folders` quirk.
- **Evidence**: extgaps QA failures (TestCopyInstallTreeMaterializesSymlinkTargets etc.); mcp-auth-security ledger; multiple Hermes review batches. (codex_ledger, local_runs)

#### S-M5. `agh-process-group-supervision`

- **Mandate**: Unix process groups; Windows forced-exit fallback. Cross-build with `GOOS=windows GOARCH=amd64 go build` before claiming subprocess work complete. Centralize signaling helpers in `internal/procutil`.
- **Evidence**: `acp-supervision` ledger (Windows fallbacks); session-stop-hang.md; PR 48 CI fix (Linux race exposure via `act`). (codex_ledger, codex_plans)

#### S-M6. `agh-truthful-ui` (or `agh-no-fakery-ui`)

- **Mandate**: UI must reflect actual backend support. No invented metrics, no plausible-looking but unmodeled controls. When Paper artboards conflict with daemon truth, daemon wins. Paper governs composition; `DESIGN.md` governs grammar.
- **Evidence**: codex_plans (`automation-bridges-paper-redesign.md`, `network-paper-pages.md`, `bridge-web-e2e.md`).

#### S-M7. `agh-hard-cut-rename`

- **Mandate**: when renaming a concept, sweep code, storage, APIs, CLI, extensions, specs, RFCs, AND `.compozy/tasks/*` artifacts simultaneously. Rewrite schema to final names. No aliases, no dual fields, no migration code.
- **Evidence**: codex_plans (`network-rename-hard-cut`, `assistant-ui-hard-cut`, `workspace-menu-hardcut`, `remove-legacy-alpha`); codex_ledger (network rename hard-cut, channel↔bridge↔space).

#### S-M8. `agh-prompt-streaming-protocol`

- **Mandate**: prompt execution detached from request context via `context.WithoutCancel`; explicit `CancelPrompt` API; AI SDK v6 UI-message parts (`tool-input-start` → `tool-input-available` → `tool-output-available`); AGH-specific data parts (`data-agh-permission`, `data-agh-event`) are additive only.
- **Evidence**: codex_plans (`prompt-stream-stall.md`, `assistant-ui-hard-cut.md`, `acp-history-replay.md`); codex_ledger (prompt-stream-stall is a four-cause incident).

#### S-M9. `agh-techspec-quality-gate`

- **Mandate**: refuse to mark a techspec ready-to-execute without 6 markers — (a) MVP boundary statement, (b) listed Architectural Boundaries, (c) Go interface signatures pasted as code blocks, (d) data-model field rationale, (e) side-table-vs-JSON decisions, (f) lease/safety invariants enumerated as numbered list.
- **Evidence**: compozy_tasks — autonomy techspec (cleanly executed across 18 tasks with 1 review round) has all 6 markers; release-adjustments and qa-review (no techspec) have unresolved review queues.

#### S-M10. `agh-shared-handler-rule`

- **Mandate**: every REST/UDS endpoint lives as a shared `BaseHandlers` method in `internal/api/core`; HTTP and UDS only choose registration and authentication. No transport-duplicated parsing/validation.
- **Evidence**: codex_plans (`hooks-cli-endpoints.md`, `api-contract-codegen.md`, `bridge-web-e2e.md`).

#### S-M11. `agh-observability-events`

- **Mandate**: every domain operation emits a canonical event with correlation keys (`workspace_id`, `session_id`, `task_id`, `run_id`, etc.). Cover with a coverage matrix test that fails if any required lifecycle path doesn't emit its canonical event.
- **Evidence**: codex_plans (`observability-spine.md` is the substrate; nearly every later plan extends events); compozy_tasks `_techspec.md` enumerates 14+ metrics + 19+ structured log fields.

#### S-M12. `agh-store-sqlite-hygiene`

- **Mandate**: `recoverSQLiteDatabase` paths rename `.db` AND `-wal`/`-shm` siblings. `BEGIN IMMEDIATE` for atomic claim/lease. Schema-version bump on every column change.
- **Evidence**: refac-v2/issue_001 (Critical WAL recovery bug); harness review flagging `CREATE TABLE IF NOT EXISTS` schema evolution. (local_runs, codex_ledger)

#### S-M13. `agh-memory-consolidation-design`

- **Mandate**: AutoDream three-gate cascade (Time → Sessions → Lock) ordered by computational cost. Forked-agent execution. Four-type memory taxonomy (`user/feedback/project/reference`); three scopes (`agent/workspace/global`). `sanitizePathKey` + `realpathDeepestExisting` for path security.
- **Evidence**: qmd_collections (RFC 002 + claude-code AutoDream article — direct ancestor). `internal/memory/consolidation/` already exists in tree.

#### S-M14. `agh-network-rfc-author`

- **Mandate**: layered RFC structure (Core / Runtime Delivery / Trust); v0/v1 wire-compat; six canonical message kinds; six lifecycle states; commit-first in-process delivery; JCS+Ed25519 verification steps; proof-stripping defense (verified-format without proof = `rejected`, not `unverified`).
- **Evidence**: qmd_collections (RFC 003-v0, RFC 004 already implement this).

#### S-M15. `agh-ecosystem-positioning`

- **Mandate**: single-source-of-truth on what AGH is NOT (workflow engine, federation protocol, MCP replacement, A2A replacement) and what it competes on (runtime/SDK/observability/DX _outside_ the open agent network protocol).
- **Evidence**: qmd_collections — RFC 003-old §4.5 explicitly states this. Critical for site/docs work. The `agh-site-*`, `agh-docs/`, `agh-compozy/` collections are empty (no public artifacts yet).

### LOW priority (specialized or narrow — consider before adoption)

| Skill                             | Rationale                                                                                       | Source          |
| --------------------------------- | ----------------------------------------------------------------------------------------------- | --------------- |
| `agh-msw-storybook-grouping`      | Web-specific, narrow — could fold into `app-renderer-systems` skill                             | codex_plans     |
| `act-linux-repro`                 | Could be a documented runbook step in `agh-process-group-supervision` instead of separate skill | codex_ledger    |
| `agh-bridge-removal-discipline`   | One-off cleanup pattern; might just be a CLAUDE.md rule                                         | codex_ledger    |
| `agh-context-budget-discipline`   | Hard to enforce mechanically — better as a CLAUDE.md rule                                       | qmd_collections |
| `agh-agent-md-author`             | Future-state (RFC 001 not yet implemented) — defer                                              | qmd_collections |
| `cy-domain-validate-method`       | Narrow request-type pattern; could be a CLAUDE.md item                                          | global_runs     |
| `cy-review-batch-sizer`           | Workflow ergonomics — could extend `cy-fix-reviews`                                             | global_runs     |
| `cy-prompt-fixture-canonicalizer` | One incident only (autonomy BUG-002)                                                            | global_runs     |

### Skills to NOT create (collisions)

- Anything writing to `.compozy/tasks/<name>/memory/` under a new name (`cy-workflow-memory` owns it)
- Alternate review-loop tooling (`cy-review-round` + `cy-fix-reviews` + `fix-coderabbit-review` is canonical)
- New `.claude/agents/*-advisor.md` archetypes (six council archetypes are intentional)
- A "code-review" or generic "audit" skill (overlaps with `architectural-analysis`, `refactoring-analysis`, `adversarial-review`, `security-review`)
- An "AGH-docs" skill (`documentation-writer` + `crafting-effective-readmes` cover docs)
- Cron/schedule-based CI skills (`feedback_ci_no_cron.md` user memory rejects this)

---

## Lesson-Learned Candidates

Sorted by source-count (cross-validated lessons first). Each is a concrete incident worth preserving.

### Validated across multiple analyses

**L1. HTTP request lifetime ≠ prompt execution lifetime.** Detach via `context.WithoutCancel(...)`; expose explicit cancel endpoints. (codex_plans, codex_ledger, global_runs, compozy_tasks)

**L2. `t.Parallel()` is incompatible with `t.Setenv`.** Always reject reviewer suggestions to add it to env-mutating tests. (codex_ledger, global_runs, compozy_tasks)

**L3. `task_runs` is the single durable queue — never add a parallel queue.** Three ADRs forbid this (autonomy 003/004/010). (compozy_tasks)

**L4. Manual control = peer to autonomous, not backdoor.** Both paths converge on same primitives (claim tokens, leases, hooks). (compozy_tasks ADR-010 — most-repeated rule)

**L5. Sub-systems may observe and notify, but durable ownership stays in the owning service.** Scheduler can wake/sweep but never claim — `task.Service.ClaimNextRun` is the only authority. (compozy_tasks ADR-004; global_runs)

**L6. Greenfield + zero-legacy means _delete_, not _adapt_.** Every breaking-change spec must explicitly name the delete target. (local_runs, codex_plans `remove-legacy-alpha.md`, codex_ledger)

**L7. E2E harness regressions follow runtime contract changes** — when prompt augmenter ships, deterministic ACP mock fixture matcher updates in the same PR. (compozy_tasks task_18 BUG-001/002/003)

**L8. Schema migrations are required even on fresh DBs.** `EnsureSchema`-style boot reconcile is forbidden for column changes. Document schema explicitly; cover with `Test*FreshDB` and reopen-after-restart. (codex_ledger, global_runs hermes BUG-002)

**L9. Concurrent worktree commits will deadlock.** `.git/index.lock` contention from concurrent task workers. Each subagent owns its own worktree. (local_runs)

**L10. `gpt-5.5` (any non-existent model name) silently breaks the entire batch.** Validate configured model against IDE's actual model list at run start. (local_runs)

### Single-source but high-leverage

**L11. The "two-touch" rule.** After two patches to the same code area, the third change must be a structural redesign, not a third patch. (codex_sessions: explicit verbatim quote)

**L12. SQLite `ORDER BY 0` bug.** Treats `0` as positional reference, not literal. Use `(SELECT 0)` or explicit constant column. (compozy_tasks task_08)

**L13. Subprocess shutdown races between health monitor and stop path.** Per-run immutable struct (`healthMonitorRun`) — never put goroutine-owned channels in a struct field that another goroutine mutates. (codex_sessions, codex_ledger)

**L14. xterm hidden/reveal lifecycle.** Measuring or fitting xterm under `display:none` produces stuck `300x150` canvases. Visibility-aware fit guard; chained `requestAnimationFrame`; `ResizeObserver` ignores zero-size. (codex_plans `dashboard-xterm-visibility.md`)

**L15. Process-group termination is mandatory for wrapper-launched ACP runtimes.** `npm exec ... -> node -> native codex-acp` keeps stdio open through descendants; `SIGTERM` alone leaves the tree alive. (codex_plans `session-stop-hang.md`)

**L16. MSW Storybook addon `resetHandlers()` reapplies only `parameters.msw`.** Flat-array handlers + per-story override = global wipe. Solve at contract level: grouped registry, override one group only. (codex_plans `storybook-route-stories-plan.md`)

**L17. Markdown is source of truth; FTS5 is derived.** Search/recall path synthesizes safe index when `MEMORY.md` is missing/stale; never silently overwrite during read. (codex_plans `memory-standard-upgrade.md`)

**L18. CGO race flag in CI.** `make verify` ran with `CGO_ENABLED=0` but `magefile.go` ran `go test -race`. Race-enabled paths self-manage cgo via `runRaceEnabledGoCommand` helper. (codex_ledger PR 35)

**L19. Replace third-party CI actions with shell logic when their setup fails on runners.** `dorny/paths-filter@v3` runner instability replaced by inline git-based change detection. (codex_ledger PR 48)

**L20. `act --container-architecture linux/amd64`** is the canonical local repro for Linux race issues that pass on macOS. Race-sensitive packages: `internal/session`, `internal/acp`, `internal/hooks`, `internal/subprocess`, `internal/resources`. (codex_ledger)

**L21. Inactivity-supervisor heartbeats must NOT flow through ACP event channel.** Backpressure risk; heartbeats update metadata only; `runtime_progress` is low-cadence persisted event. (codex_plans `long-running-sessions.md`)

**L22. Capability-published-but-never-matched methodology.** Eight of ten parallel research slices independently flagged the same six source lines. Lesson: when multiple investigations converge on same data structure as "right shape but unconsumed," the gap is integration, not architecture — _build the consumer; don't redesign the data_. (global_runs, autonomous analysis)

**L23. Daemon round-3 retry storm pattern.** When `cy-fix-reviews` doesn't converge in 1-2 rounds for a given file, the underlying issue is structural (mock contract drift, schema mismatch, hidden invariant) — needs human triage, not more agent loops. 19 attempts was invisible from inside any single attempt. (global_runs)

**L24. AutoDream gates, not heuristics.** Time → Sessions → Lock cascade ordered by cost prevents both over-consolidation (cost) and races (corruption). Never replace with naive "consolidate at session end." (qmd_collections)

**L25. Proof-stripping is a real attack class.** Verified-format identity (`nickname@fingerprint`) without valid `proof` MUST classify as `rejected`, not `unverified`. (qmd_collections RFC 004)

**L26. Defer crypto to ship.** RFC 003 wire envelope shipped as v0 (no crypto) → v1 (Baseline Trust Profile) without breaking wire compat. Wire-compatibility-first beats crypto-first. (qmd_collections)

**L27. "Format, not runtime" trap.** AgentSkills/AGENTS.md/A2A Agent Cards are file formats without runtime governance. AGH's pattern: extend (not fork) the format AND add the runtime. Always ask "is the upstream a format or a runtime?" before composing vs. replacing. (qmd_collections)

**L28. Five-layer precedence is the right number.** Skills/memory/agent: Bundled → Marketplace → User → Additional → Workspace. Six is too many; three loses Marketplace vs User trust tiers. (qmd_collections)

**L29. AGH was 80% built before the autonomy program.** The autonomy work was _integration_, not _invention_. Surfaces a methodology: when scoping a new program, count what already exists. (global_runs analysis intro)

**L30. One QA pass surfaces cross-component drift `make verify` misses.** Three E2E regressions (BUG-001/002/003) all rooted in test-vs-runtime contract drift, not production bugs. (compozy_tasks task_18, global_runs)

---

## System Prompt Candidates for CLAUDE.md

Sorted by **evidence weight** (how many analyses support it). The strongest are at the top. Each is phrased as Pedro could paste it into CLAUDE.md.

### Tier 1 — Highest weight (4+ analyses agree)

**P-T1.1 (test discipline)**: _"Every Go test case MUST be inside a `t.Run("Should ...")` subtest. Independent subtests MUST call `t.Parallel()`. The only legitimate opt-out from `t.Parallel()` is a comment justifying `t.Setenv` or shared state — reject reviewer suggestions to add `t.Parallel()` to env-mutating tests as INVALID."_
Promote the existing soft "table-driven default" wording to a hard rule. Reviewers literally quote CLAUDE.md back at code that ignores this.

**P-T1.2 (no-`_`-error)**: Promote _"Never ignore errors with `_`— every error must be handled or have a written justification"_ to a`<critical>` block. Already in CLAUDE.md but agents still violate it.

**P-T1.3 (errors.Is/As)**: _"Use `errors.Is`/`errors.As` exclusively for error matching. `strings.Contains(err.Error(), …)` is forbidden."_

**P-T1.4 (greenfield deletion)**: _"Every breaking-change techspec must explicitly name its delete targets. 'Delete the old thing' is not a default; it is a checklist item that must be enumerated."_

**P-T1.5 (codegen co-ship)**: _"Generated artifacts ship in the same PR as their source. After any change to `openapi/agh.json`, `openapi/compozy-daemon.json`, `internal/api/contract/**`, or any DTO: run `make codegen`, verify `make codegen-check`, run `make web-typecheck` and `make web-test`. Web fixtures and tests update in lockstep."_

**P-T1.6 (schema migrations)**: _"Any change to a SQLite column, index, or constraint MUST add a versioned migration in the migrations registry. `EnsureSchema`-style boot reconciliation is forbidden for column changes. Test fresh-DB and reopen-after-restart paths."_

### Tier 2 — Strong (2-3 analyses agree)

**P-T2.1 (subagents read-only)**: _"Subagents are for analysis and exploration only — never for implementation. The author of every code change is the agent paired with the user; subagent output is treated as evidence, not as committed work."_ (codex_sessions critical block)

**P-T2.2 (auto QA pair)**: _"Every `cy-create-tasks` run produces two trailing tasks: `$qa-report` + `$qa-execution` following the `.compozy/tasks/hermes` template. UI-bearing features include e2e (Playwright or browser-use) in qa-execution."_

**P-T2.3 (cross-LLM techspec review)**: _"Before approving any TechSpec or major architecture change, run a peer review through `compozy exec --ide claude --model opus --reasoning-effort xhigh --format json`. Codex authors specs; Claude Opus pressure-tests them; gpt-5.4-mini-high explores breadth in parallel subagents. Do not substitute models without explicit user approval."_

**P-T2.4 (web/docs impact)**: _"Every backend feature task includes a `Web/Docs Impact` subitem listing affected `web/` routes/components/hooks AND `packages/site` doc pages. Backend-only tasks may declare 'no impact' but only after analysis."_

**P-T2.5 (worktree isolation)**: _"When the user runs multiple AGH/Compozy agents in parallel worktrees, every test or QA run uses a unique `AGH_HOME` and unique daemon ports. Default home and default port use is forbidden in QA flows when concurrency is signaled."_

**P-T2.6 (detached prompt lifetime)**: _"Any work that outlives an HTTP/UDS request — prompts, network channel sends, automation jobs — MUST detach via `context.WithoutCancel(...)`. Never tie execution lifetime to request lifetime. Expose explicit cancel endpoints (e.g., `POST /api/sessions/:id/prompt/cancel`)."_

**P-T2.7 (`task_runs` exclusivity)**: _"`task_runs` is the only durable work queue. Do not introduce a parallel queue or actor table. Add new ownership/state via columns + side tables on `task_runs`."_

**P-T2.8 (authoritative primitives)**: _"When an authoritative primitive owns a state transition (`ClaimNextRun`, `Spawn`, `EnsureMigration`), no peer package may replicate the transition. Wake/observe/sweep are allowed; claim/own is not. The mechanical scheduler does not call `ClaimNextRun`."_

**P-T2.9 (manual = peer)**: _"Manual operator paths and autonomous paths converge on the same primitives. User-created tasks, automation-created tasks, coordinator-created tasks, and agent-created child tasks all use the same task/run model and the same claim-token/lease/heartbeat/complete/fail/release rules. Task creation alone NEVER enqueues claimable work or starts the coordinator; publish/start/approval is the run-enqueue boundary."_

**P-T2.10 (hooks ≠ event bus)**: _"Hooks dispatch at the call site that owns the state transition. Never tail event/log tables to fire hooks. Hooks may deny/narrow/annotate but cannot bypass `ClaimNextRun`, lease tokens, TTL, lineage, spawn caps, or permission narrowing."_

**P-T2.11 (external-call timeouts)**: _"Outbound HTTP/network calls MUST use a client with an explicit timeout. `http.DefaultClient` is forbidden in production code paths."_

**P-T2.12 (CLI flag presence)**: _"CLI/handler logic MUST distinguish 'flag not set' from 'flag set to zero value'. Use `cmd.Flags().Changed(name)` (Cobra) or equivalent presence detection. Silently ignoring an explicit flag is a bug."_

**P-T2.13 (whitespace normalization)**: _"String-slice CLI inputs (capabilities, IDs, tags, paths) MUST trim and drop empty entries before sending. Do not push whitespace-only strings to the daemon as 'validation problems'."_

**P-T2.14 (interface assertions)**: *"`var \_ Interface = (*Type)(nil)` is mandatory next to every new exported type that satisfies an interface."\*

**P-T2.15 (one commit per remediation batch)**: _"Each `cy-fix-reviews` round produces exactly one local commit. Run `make verify` BEFORE and AFTER the commit. Never `git commit --amend` after pre-commit hook failures — fix and create a new commit."_

**P-T2.16 (race + cgo)**: _"Race-enabled tests must self-manage `CGO_ENABLED=1`. Verification commands wrapping `go test -race` go through `runRaceEnabledGoCommand` (or equivalent). Don't trust ambient env."_

**P-T2.17 (Linux-race CI parity)**: _"Before claiming `make verify` complete on race-sensitive packages (`internal/session`, `internal/acp`, `internal/hooks`, `internal/subprocess`, `internal/resources`), reproduce locally with `act workflow_dispatch -W .github/workflows/ci.yml -j verify --container-architecture linux/amd64`."_

**P-T2.18 (secret redaction non-negotiable)**: _"`claim_token` (`agh*claim*_`), MCP auth tokens, OAuth codes, PKCE verifiers, and secret bindings MUST NEVER appear in logs, status APIs, settings views, error payloads, channel messages, SSE, web UI, or memory. Use hash forms (`claim_token_hash`) over the wire."\*

**P-T2.19 (symlink-escape hardening)**: _"Skill sidecars, skill files, managed-extension copies, and bundle install paths MUST verify resolved targets remain inside approved roots. Use `EvalSymlinks` + path-prefix check, not naive joins."_

**P-T2.20 (process-group parity)**: _"Subprocess work uses Unix process groups + Windows forced-exit fallback. Cross-build with `GOOS=windows GOARCH=amd64 go build` before claiming subprocess work complete. Centralize signaling helpers in `internal/procutil`."_

**P-T2.21 (hard-cut renames)**: _"Renames sweep code, storage, APIs, CLI, extensions, specs, RFCs, and `.compozy/tasks/_` artifacts in the same change. No aliases, no dual fields, no schema fallback paths."\*

**P-T2.22 (forensic-first bug fixes)**: _"Bug-fix plans open with a confirmed reproduction (timestamp, command, observed evidence) before listing changes. 'I think' or 'probably' is forbidden at the top of a fix plan."_

**P-T2.23 (no partial-surface completions)**: _"Any change touching a public surface must close the loop end-to-end in one pass: contract → HTTP handler → UDS handler → CLI client → CLI command → tests → docs. Stopping at 'HTTP works, UDS later' forces a re-plan."_

**P-T2.24 (composition-root)**: _"Only `daemon/` wires components. Reconciliation logic running at boot belongs to composition root and is not 'legacy support'."_

**P-T2.25 (`internal/api/core` canonical)**: _"REST/UDS endpoints exist as shared `BaseHandlers` methods in `internal/api/core`; HTTP and UDS only choose registration and authentication."_

**P-T2.26 (truthful UI)**: _"UI must reflect actual backend support. Don't render controls or metrics the runtime doesn't model. When Paper artboards conflict with daemon truth, daemon wins. Paper governs composition; `DESIGN.md` governs grammar."_

**P-T2.27 (ledger maintenance)**: _"Read existing `.codex/ledger/` files for cross-agent awareness before starting; create a session ledger when working on a multi-step task; update Done/Now/Next as work progresses. Don't commit `.compozy/tasks/_/memory/` files in code commits — tracking artifacts stay local unstaged."\*

**P-T2.28 (`make verify` is commit gate)**: _"`make verify` is the commit gate. If verification is blocked by an external/branch-side asset issue (missing test fixture, etc.), do NOT commit — report the verified blocker and hold."_

**P-T2.29 (test failures are production bugs)**: _"Test failures uncovered by verification are production bugs, not test issues. Fix production code; don't weaken assertions. The only exception is documenting an INVALID review item with concrete evidence."_

**P-T2.30 (memory consolidation gates)**: _"Background consolidation runs only when all gates pass in order of computational cost: Time → Sessions → Lock. Default gates: 24h, 5 touched sessions, file-lock. Never replace gates with naive heuristics."_

### Tier 3 — Single-source but architecturally important

**P-T3.1 (two-touch rule)**: _"If the same package or behavior has been patched twice in the same workstream, the third change MUST be a structural redesign, not a third patch. Schedule the redesign explicitly via a new TechSpec."_ (codex_sessions verbatim)

**P-T3.2 (`exec` headless default)**: _"`compozy exec` is a headless command. `--format text` returns a single string; `--format json` returns a stream of valid JSON objects; the TUI is opt-in via `--tui`. `exec` does not persist artifacts to `.compozy/runs/` unless `--persist` is given."_ (codex_sessions explicit correction)

**P-T3.3 (competitor refs in tasks)**: _"When a TechSpec relies on `.resources/<repo>` references, each generated task includes explicit file paths from those competitors so the implementing agent reads them too. Reference-bearing analysis files belong under `.compozy/tasks/<slug>/analysis/`."_ (codex_sessions repeated)

**P-T3.4 (CLI verb identity)**: _"Agent-facing CLI commands stay identity-inferred from `AGH_SESSION_ID` / `AGH_AGENT` via `internal/agentidentity`. Operator endpoints MUST NOT infer agent identity from environment variables."_ (compozy_tasks ADR-002)

**P-T3.5 (permission narrowing atoms)**: _"Permission narrowing compares concrete atoms only: tools, skills, MCP server IDs, workspace path grants, network channels, env profile grants. Subset-only; unknown child atoms count as widening and reject the spawn."_ (compozy_tasks ADR-006)

**P-T3.6 (boot recovery before scheduler)**: _"Boot recovery runs BEFORE the scheduler accepts wake/claim traffic. A session may hold at most ONE active task-run lease in MVP. The reaper releases leases before stopping a child session."_ (compozy_tasks `_techspec.md`)

**P-T3.7 (`context.WithoutCancel` does not preserve deadlines)**: Add as Go-specific gotcha next to existing concurrency rules. (compozy_tasks hermes round-2 issue 001)

**P-T3.8 (defensive nil-check after `make`)**: _"Do not add `if x == nil` guards on values just initialized with `make(...)`. Reviewers and lint flag these as unreachable."_ (compozy_tasks)

**P-T3.9 (capability vs recipe vocabulary)**: _"Reusable agent artifacts are called `capability`, NOT `recipe`/`workflow`/`procedure`/`playbook`. Capabilities are interpretive, not deterministic; they are not workflow programs in disguise."_ (qmd_collections)

**P-T3.10 (proof-stripping defense)**: _"In any signed-message processing path, an identity in verified format (`nickname@fingerprint`) without valid `proof` MUST classify as `rejected`, not `unverified`."_ (qmd_collections RFC 004)

**P-T3.11 (5-layer precedence)**: _"Skill, memory, and agent resolution use a 5-layer precedence: Bundled → Marketplace → User → Additional → Workspace, with agent-local overriding all. Higher precedence wins on collision; an audit trail logs every shadow."_ (qmd_collections)

**P-T3.12 (load-time security scan)**: _"Every non-bundled skill is scanned via `VerifyContent` on every load (not just install). Critical findings block; warning findings log; info findings log silently. Bundled skills are exempt because `go:embed` provides immutability."_ (qmd_collections)

**P-T3.13 (memory taxonomy)**: _"User-facing memories use the four-type taxonomy `user | feedback | project | reference`, written to scopes `agent | workspace | global`. The default write scope is declared per agent in `memory.scope`."_ (qmd_collections)

**P-T3.14 (path security helpers)**: _"Filesystem helpers resolving user-controlled or agent-controlled paths use the `sanitizePathKey` + `realpathDeepestExisting` pattern (defenses against null-byte, URL-encoded traversal, Unicode normalization, symlink-escape)."_ (qmd_collections)

**P-T3.15 (runtime moat)**: _"AGH's competitive surface is runtime, SDK, observability, DX, and integration depth — NOT the open agent network protocol. AGH Network must remain implementable outside AGH. Any feature requiring AGH to interoperate is a design smell."_ (qmd_collections)

**P-T3.16 (canonical ledger format)**: _"Ledger files use the canonical `Goal / Constraints / Decisions / State / Done / Now / Next / Open Questions / Working set` format. The 'Working set' section captures exact file paths + commands."_ (codex_ledger)

---

## Stale CLAUDE.md Items to Fix

Pulled from `analysis/analysis_existing_surfaces.md`:

1. **Phase ordering is outdated.** CLAUDE.md says phases are 1) Agent core → 2) Memory/Skills/State → 3) Network protocol. Reality: network exists today; autonomy kernel is the current focus.

2. **Package layout table is materially out of date.** Missing: `internal/scheduler`, `internal/agentidentity`, `internal/situation`, `internal/hooks`, `internal/task`, `internal/network`, `internal/resources`, `packages/site`.

3. **Build commands are incomplete.** Missing: `make codegen`, `make codegen-check`, `make test-e2e-web`, `make test-e2e-nightly`, `make test-integration`, `cd packages/site && bun run source:generate`, `bun run typecheck`, `bun run test`.

4. **`nats` skill in installed catalog vs. forbidden by architecture.** Note that the dispatch table correctly omits it.

5. **`cy-final-verify` row understates verification scope.** Real flow now also runs `qa-report` + `qa-execution` (and now `real-scenario-qa`).

6. **Skill dispatch table is missing**: `real-scenario-qa` (just added project-local), the layered QA stack (`qa-execution` + `qa-report` + `real-scenario-qa`), `compozy` skill, `cy-workflow-memory`, the `impeccable:*` family.

7. **Web search tools row is too coarse.** Multiple tools available: `find-docs`, `context7`, `exa-web-search-free`, `firecrawl:firecrawl-cli`. Pick by purpose.

8. **AGENTS.md vs CLAUDE.md divergence.** Only `<critical>NEVER COMMITS ai-docs/ TO THE REPO</critical>` differs. Consider promoting to CLAUDE.md or the divergence rule should be explicit.

9. **No mention of the `<critical>NEVER COMMITS .compozy/tasks/*/memory/ TO THE REPO</critical>` rule** that the codex_ledger pattern shows is enforced in practice.

10. **Site subtree (packages/site) lacks a CLAUDE.md or AGENTS.md** despite being a third surface peer to Go backend and React web.

---

## Other Things Worth Discussing

### About user memory (`MEMORY.md`)

The user-memory file at `~/.claude/projects/-Users-pedronauck-Dev-compozy-agh/memory/` is silent on:

- The autonomous mode kernel (ADRs 001-012)
- The manual-first contract
- The QA pattern from `real-scenario-qa`
- Coordination channels / claim-lease / safe spawn / hook taxonomy
- The greenfield-alpha "delete don't adapt" working rule
- The two-touch rule
- The five-layer skill/memory precedence

If we don't promote any of the above to CLAUDE.md, several are good user-memory candidates (they're project-shape facts that persist across conversations).

### About the empty agh-\* QMD collections

`qmd://agh-compozy/`, `qmd://agh-docs/`, `qmd://agh-site-archived/`, `qmd://agh-site-ledger/`, `qmd://agh-site-plans/` are all 0 files. The Fumadocs site project (per existing memory `project_site_docs.md`) is approved but not yet indexed. We should either populate these collections or remove them from QMD to avoid confusion.

### About the `agh-rfcs-local` collection

5 RFCs (~125KB total). RFC 002 is partly retrospective of code already in tree (`internal/skills/`, `verify_test.go`, `provenance.go`, `mcp.go`, `mcp_sidecar.go`, `hook_decl.go`). RFCs 003-v0 and 004 use `capability`; the older 003 still uses `recipe`. The corpus would benefit from a canonical glossary.

### Cross-cutting "what to check next time"

- **Compozy lacks a "convergence health" signal.** The 19-attempt daemon round-3 retry storm was invisible from inside any single attempt. Could be a Compozy emit metric: "this batch needed N attempts to drain."
- **Two undated standing directives** (`long-running-sessions.md`, `remove-legacy-alpha.md`) act as ongoing posture rather than per-task plans. Candidates for a `docs/_memory/standing_directives.md`.
- **Brazilian Portuguese** is conversational; artifacts (TechSpecs, ADRs, code, commit messages) are always English. This is a discoverable rule worth codifying.

---

## Recommended Synthesis Plan

If Pedro approves, the next steps would be (in order):

1. **Update CLAUDE.md** with the Tier-1 system prompts, fixed package layout table, fixed build commands, fixed phase framing.
2. **Update `MEMORY.md`** with project-shape facts (autonomy kernel, manual-first contract, two-touch rule, ledger maintenance) and feedback-shape rules (subagents read-only, BR-PT conversation/EN artifacts).
3. **Create new skills via `/writing-skills`** in this order: HIGH-priority workflow skills first (`cy-tasks-tail-qa-pair`, `cy-spec-peer-review`, `cy-web-docs-impact`, `agh-worktree-isolation`), then HIGH-priority code-discipline skills (`agh-test-conventions`, `agh-cleanup-failure-paths`, `agh-schema-migration`, `agh-contract-codegen-coship`).
4. **Capture lesson-learned candidates** in a `docs/_memory/lessons/` registry — start with the Tier 1 (multi-source) lessons.
5. **Decide on standing directives doc** (`docs/_memory/standing_directives.md`) for `long-running-sessions` and `remove-legacy-alpha`.
6. **Decide on AGH glossary** to lock down `capability` vs. `recipe` vs. `skill`, AGENT.md vs. AGENTS.md, AGH-Network Peer Card vs. A2A Agent Card, and the "what AGH is not" list. Belongs on the marketing site.
