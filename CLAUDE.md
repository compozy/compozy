## Project Overview

AGH is an agent operating system: a Go single-binary daemon that manages AI agent sessions via ACP — spawning ACP agents (Claude Code, OpenClaw, Hermes) as subprocesses over JSON-RPC/stdio, persisting events in SQLite, exposing HTTP/SSE (web UI) + UDS (CLI). Docs live at `agh.network` (Fumadocs).

**Core premise:** every capability must be both extensible by the runtime and manageable by agents (CLI/HTTP/UDS with structured output). A feature that only works through internal Go calls or the web UI is incomplete.

## Greenfield Alpha — Zero Legacy Tolerance

No production users. Never sacrifice quality for backward compatibility; never write migration/compat/defensive code for old state — **delete obsolete code instead**. Hard cuts, not bridges: a rename updates code, storage, APIs, CLI, extensions, specs, RFCs, and `.compozy/tasks/*` in one change — no aliases, dual fields, or schema fallbacks. Every breaking-change techspec MUST list its delete targets.

## Critical Rules

- **`make verify` MUST pass before completing any task** (full monorepo gate — see Build Commands). Zero warnings, zero errors. Exception: docs-only changes that don't affect test/lint/typecheck.
- **`make lint` and `make bun-lint` are zero-tolerance** — any warning is a blocking failure.
- **Check dependent package APIs** before writing integration code or tests.
- **Never hand-edit `go.mod`** — use `go get`. **Never hand-add JS deps** — use `bun add`.
- **Never run destructive git** (`restore`, `checkout`, `reset`, `clean`, `rm`, `stash`) without explicit user permission. Work around unexpected worktree edits; don't touch files you didn't change.
- <critical>NEVER discard errors with `_` in production or tests — handle every error or write a justification.</critical>
- <critical>**No god files — one responsibility per file, hard cap 500 lines for production source (tests excluded).** Never mix domain types + registry/wiring + multiple implementations + generic helpers in a single file: `internal/loop/action.go` landing at 1380 lines (4 executors + registry + schema validation + JSON extraction + template rendering) is the canonical violation. Decide the file split BEFORE writing: contract/types, registry/options, one implementation per file, cross-cutting helpers in their own named file. Creating a file over the cap — or growing one past it — is a blocking architecture failure: split it in the same change; "it's all related" is never a justification. Files already over the cap must not grow — extract into a new file instead of appending.</critical>
- <critical>**Context-budget docs stay lean.** `CLAUDE.md`/`AGENTS.md` and `SKILL.md`s load into prompts — every line costs every turn. Before editing agent instruction files activate `writing-agents-md`; before editing skills activate `writing-skills`. Growing either with restated or redundant prose is a blocking failure.</critical>
- **Test placement before test creation** (skill: `consolidate-test-suites`). Name the invariant, owning layer, and canonical suite; edit the existing suite — don't create standalone/duplicate regressions. Static/prose/CSS/snapshot/generated/config tests are forbidden by default: allowed only when that artifact is the product contract and no stronger gate (`make verify`, `codegen-check`, build, link-check, Storybook capture) owns it.
- **Subagents for exploration** (keeps your context clean): Use your native subagents tool to explore; Just use `agent-exploration` when required

## Workflow Rules

- **TechSpec peer review is opt-in, after draft approval.** `cy-create-techspec` presents + saves the approved draft first, then offers `cy-spec-peer-review`. Apply only user-selected findings.
- **Every backend task carries a `Web/Docs Impact` subitem** — affected `web/` routes/components/hooks AND `packages/site` docs. "No impact" only after analysis.
- **Every spec/feature carries an extensibility + agent-manageability + config-lifecycle analysis** — how it wires into extension surfaces (extensions, hooks, skills/capabilities, tools/resources, bundles, registries, bridge SDKs), which CLI/HTTP/UDS surfaces let agents manage it, and which `config.toml` keys/defaults/docs change. "No impact" needs explicit evidence.
- **Reference competitors by file path in tasks.** `.resources/<repo>/`-backed tasks list explicit competitor paths; analysis files go under `.compozy/tasks/<slug>/analysis/`.
- **Worktree isolation is mandatory for parallel QA** — unique `AGH_HOME`, daemon ports, and `tmux-bridge` sockets. Default home/port is forbidden when concurrency is signaled.
- **Deterministic QA bootstrap for local release/scenario QA** — start with `agh-qa-bootstrap`; fresh lab per pass; reuse a `bootstrap-manifest.json` only when continuing the same active QA loop. QA state lives in the committed `docs/qa/` tree (`scenarios/*.md`, content-addressed bugs, journeys, charters, dated reports); `state.csv` is a gitignored generated view, and the lab holds only run-scratch evidence indexed by path.
- <critical>**QA process teardown is mandatory (L-029).** Every QA lab or isolated runtime envelope ends with `eval "$TEARDOWN_COMMAND"` (from the bootstrap manifest) or `make qa-reap` — on every terminal path (pass/fail/blocked/abort). Files may stay for forensics; processes never do. Completing a task while lab daemons, tmux servers, dev servers, browsers, or watchers are still alive is a blocking failure; cite `teardown.json` (`"clean": true`) as evidence. Register long-lived lab processes at `<QA_OUTPUT_PATH>/qa/pids/<name>.pid` on spawn.</critical>
- **QA tracker impact flag before completing any task** — if the diff changes user-visible behavior (UI, CLI verb, API route, config key, copy), flag it in `docs/qa/scenarios/`: new behavior → add an `untested` content-addressed scenario file; changed behavior → reset the affected file's `qa_status` to `untested`. Pure refactors declare "no user-visible change". **Flag, don't retest** — retests belong to the next QA cycle (`untested` scenarios ARE its scope). Use content-addressed ids for new scenarios and bugs; dedup same-behavior/same-symptom add/add conflicts instead of coordinating a shared counter.
- **Provider-home policy matches the provider contract in local QA.** Bound-secret/brokered creds use `PROVIDER_HOME`/`PROVIDER_CODEX_HOME` from the bootstrap manifest. Exception: `native_cli` + `home_policy = operator` preserves the operator `HOME`/native login unless a scenario tests isolated provider-home.
- **Isolated Web QA exports `AGH_WEB_API_PROXY_TARGET`** — derive it from the bootstrap manifest/env; never hardcode `:2123`.
- **Never parallelize config writes against one isolated QA home** — `agh config set` and peers run sequentially per provider/runtime home.
- **Skill helpers use explicit repo-root paths** (`.agents/skills/<skill>/scripts/`), never ambiguous `scripts/...`.
- **Two-touch rule.** After two patches to the same package/behavior in one workstream, the third change MUST be a structural redesign in a new TechSpec, not a third patch.
- **Conversation in Brazilian Portuguese; artifacts in English** (TechSpecs, ADRs, code, commits, docs).

## AGH Cross-Surface Impact Audit

Every feature, bug fix, refactor, public-contract/CLI/API/native-tool/config/docs change, or runtime behavior change MUST include this audit in the plan/task/completion notes. Purely editorial docs that describe no runtime behavior may state `not applicable — editorial only`.

```markdown
AGH Impact Audit:

- Native tools: <changed tool IDs/toolsets/descriptors/schema digests/capability gates/tests, or no impact + checked surfaces>
- Extensibility and hooks: <extensions, hooks, skills/capabilities, tools/resources, bundles, registries, bridge SDKs, MCP sidecars, config lifecycle, or no impact + checked surfaces>
- Workspace data isolation: <global/workspace/session/agent scope + workspace_id propagation through CLI/HTTP/UDS/core/store/web/SSE/cache/events + tests, or no impact + checked surfaces>
- Official AGH skill: <skills/agh/ updates, or no impact + checked surfaces>
```

- `No impact` is valid only when it names the exact checked surfaces and why they're unchanged.
- **Native tools** = `agh__*` IDs, toolsets, descriptors, I/O schemas, digests, risk flags, availability diagnostics, capability gates, CLI/API fallbacks.
- **Workspace data isolation** = runtime data ownership (not QA/worktree isolation): classify each new/changed datum as global/workspace/session/agent-scoped and prove list/read/cache/SSE/event paths can't leak across workspaces.
- **Official AGH skill** updates are required when public behavior, tool IDs, CLI paths, hook events, capabilities, bundles/resources, or memory/network/task semantics change. Canonical bundled skill: `skills/agh/`.

## Design System

`packages/ui/src/tokens.css` is the canonical token source; `DESIGN.md` (repo root) is its generated spec + rationale. Full grammar (flat depth model, signal palette, type stack) lives in `DESIGN.md` — pull from there, never invent.

- <critical>**Reuse before create (any UI surface):** `packages/ui/src/index.ts` is the primitive inventory — check it before authoring any component and import from `@agh/ui` instead of redefining. Shadowing an exported name in `web/`/`packages/site` is a blocked lint error (`compozy-ui-reuse/no-shadow-ui-primitive`); domain variants take domain-prefixed names (`SessionToolCallRow`); new generic primitives land in `packages/ui` (story + test), domain composites in `web/src/systems/<domain>/`.</critical>
- Pull every color/type/radius/spacing/motion value from `tokens.css` + `DESIGN.md`. Signal palette is information, never decoration: `#E8572A` action · `#5FBF85` success · `#E0635A` danger · `#D6A647` warning · `#8E8EB5` info.
- Never hand-edit `DESIGN.md` frontmatter or `<!-- BEGIN/END:tokens:* -->` regions. After changing runtime/site theme tokens run `make codegen`; `make codegen-check` enforces drift. Site-only extensions go in `packages/site/app/global.css` `@theme inline`.
- **Truthful UI > plausible UI.** Never render controls/metrics the runtime doesn't support. On conflict, daemon truth wins.
- **Design-system/redesign work runs through the `designer` agent in execution mode only** and MUST activate `agh-design` + `ui-craft` (reference-routed — read the matched rows in full). **Verify every `web/` or `packages/ui/` UI change with `agh-ui-screenshot` before completion** and cite the capture; when a spec names a visual reference, Visual Contract Mode requires a rendered reference/implementation bundle with zero unresolved structural mismatches — an implementation-only capture is not parity evidence. Reference parity binds visual language only — a prototype is lossy on content, data, copy, and brand marks; runtime truth, `COPY.md`, and the `@agh/ui` brand inventory own those axes (divergences become authorized deltas, never new brand variants or invented content).

## Copy System

`COPY.md` (repo root) is the authoritative product-language spec for all public text (marketing, docs, release, metadata, UI microcopy, CLI help, SEO/OpenGraph). Read it before writing/changing product-facing copy.

- Runtime truth beats copy preference: generated API/CLI refs, code, tests, and release artifacts win over aspirational wording.
- Follow `docs/_memory/glossary.md`. The canonical artifact name is `capability` — never `recipe`, `workflow`, `procedure`, or `playbook`.
- Apply the `COPY.md` claim standards before "today", "shipping", "supported", "live", "complete", or product counts.

## Skill Dispatch

<critical>**ALWAYS** activate skills **before** writing code.</critical> Match task domain → activate all required skills. Multiple domains → activate multiple. No skipping "because it's small".

| Domain                                            | Required Skills                                                                          | Conditional Skills                    |
| ------------------------------------------------- | ---------------------------------------------------------------------------------------- | ------------------------------------- |
| Go / Runtime                                      | `agh-code-guidelines` + `golang-pro`                                                     | `context7`                            |
| Config / Logging                                  | `agh-code-guidelines` + `golang-pro`                                                     |                                       |
| TUI / CLI Bubbletea                               | `bubbletea` + `agh-code-guidelines` + `golang-pro`                                       |                                       |
| Bug fix                                           | `systematic-debugging` + `no-workarounds`                                                | `testing-boss`                        |
| Writing Go tests                                  | `agh-test-conventions` + `testing-boss` + `golang-pro`                                   | `vitest` (only for test tooling docs) |
| Test placement / consolidation                    | `consolidate-test-suites`                                                                | `testing-boss`                        |
| Cleanup / failure paths                           | `agh-cleanup-failure-paths` + `agh-code-guidelines` + `golang-pro`                       |                                       |
| Schema / migration changes                        | `agh-schema-migration` + `golang-pro`                                                    |                                       |
| Contract / OpenAPI changes                        | `agh-contract-codegen-coship`                                                            |                                       |
| Task completion                                   | `deslop` + `cy-final-verify`                                                             |                                       |
| Lessons learned                                   | `lesson-learned`                                                                         |                                       |
| Architecture audit                                | `architectural-analysis`                                                                 | `refactoring-analysis`                |
| Concurrency / races                               | `golang-pro` + `systematic-debugging`                                                    | `agh-code-guidelines`                 |
| AGH Network (`internal/network` only)             | `agh-code-guidelines` + `golang-pro`                                                     | `systematic-debugging`                |
| Performance / hot paths                           | `extreme-software-optimization` + `golang-pro`                                           |                                       |
| Security review                                   | `security-review`                                                                        |                                       |
| Creative / new features                           | `grill-me`                                                                               |                                       |
| PRD creation                                      | `cy-spec-preflight` + `cy-create-prd`                                                    | `grill-me`                            |
| TechSpec creation                                 | `cy-spec-preflight` + `cy-create-techspec`                                               | `cy-spec-peer-review`                 |
| Task generation                                   | `cy-spec-preflight` + `cy-create-tasks` + `cy-tasks-tail-qa-pair` + `cy-web-docs-impact` |                                       |
| Research → executable issue backlog               | `cy-research-issues`                                                                     | `consolidate-test-suites`             |
| Execute a PRD task                                | `cy-execute-task`                                                                        | `cy-workflow-memory`                  |
| Review round / fixes                              | `cy-review-round` + `cy-fix-reviews`                                                     |                                       |
| Release / scenario QA                             | `agh-qa-bootstrap` + `real-scenario-qa` + `qa-report` + `qa-execution`                   | `agh-worktree-isolation`              |
| Git rebase / conflicts                            | `git-rebase`                                                                             |                                       |
| External docs lookup                              | `context7`                                                                               | `exa-web-search-free`                 |
| Parallel multi-area research                      | `agent-exploration`                                                                      |                                       |
| Diagrams (spec / ADR)                             | `mermaid-diagrams`                                                                       |                                       |
| Documentation (internal)                          | `documentation-writer`                                                                   |                                       |
| Copy / public product language                    | `copywriting` + `documentation-writer`                                                   | `seo-audit`                           |
| Skill authoring                                   | `writing-skills`                                                                         |                                       |
| Agent instruction files (`CLAUDE.md`/`AGENTS.md`) | `writing-agents-md`                                                                      |                                       |
| UI / Design (any surface)                         | `agh-design` + `ui-craft` + `impeccable`                                                 | `agh-ui-screenshot`                   |
| UI verification / visual diff                     | `agh-ui-screenshot` + `impeccable`                                                       |                                       |

Web-specific dispatch: `web/CLAUDE.md`. Site-specific: `packages/site/CLAUDE.md`.

## Build Commands

`make verify` is the only gate that exercises the entire monorepo — `codegen-check → installer-check → Bun lint → Bun typecheck → Bun test → web build → Go fmt → Go lint → Go test → Go build → boundaries`.

**Run the full `make verify` once, as the completion/PR gate — not per micro-task, not twice per commit.** A full run fans one `-race` test binary per package across every core and takes minutes; running it on every small change needlessly saturates the machine. During iteration, gate only the lane you touched; reserve `make verify` for when the task is done. This scopes the dev loop _before_ the final gate — `cy-final-verify` still requires the full pipeline (no subset) at completion.

**`make verify` and the E2E lanes self-serialize across worktrees (L-030).** These gates are machine-sized by design; two at once collapse the machine and both stall. They share a machine-wide lock (`~/Library/Caches/agh-dev/verify.lock`): a second concurrent run queues with explicit "waiting for pid N (worktree X)" messages instead of silently thrashing. Never kill a queued run assuming it hung — read its output. `AGH_VERIFY_LOCK=off` bypasses (CI-style single-checkout machines only). Scoped lanes stay lock-free but bounded: unit `go test` budgets combined package/subtest concurrency against half the effective Go CPU capacity (`AGH_GO_TEST_P` overrides the package cap), and Vitest pools cap at 50%.

- **Go change** → `make lint` + `go test -race ./internal/<pkg>/...` (scoped path, never `./...`).
- **Web / `packages/ui` / site change** → `bunx turbo run lint typecheck test --filter=./web` (or `./packages/ui`, `./packages/site`).

**Frontend validation MUST run through Turborepo from the repo root.** Never use `cd web && bun run test`, `bun run --cwd web test`, `cd packages/site && bun run …`, or package-local equivalents as evidence — they bypass Turbo's cache/task graph.

```bash
make bun-lint / bun-typecheck / bun-test   # repo-root Bun gates (oxfmt+oxlint / turbo typecheck / turbo test)
make lint                                  # strict Go + monorepo Bun lint (zero issues)
make test / test-integration               # Go unit (-race) / +integration tag
make test-e2e-runtime / test-e2e-web       # daemon-side (Go harness) / browser-side (Playwright)
make build / codegen                       # compile binary / regen openapi + TS types + DESIGN.md tokens
make worktree-new SLUG=<slug>              # sibling worktree + bootstrap (BRANCH=/BASE=/BUILD=1/E2E=1; rm via scripts/worktree.sh rm)
```

Web-local dev/build/format (`make web-dev`, `make web-build`, `make web-fmt`) are documented in `web/CLAUDE.md`.

## Commit style

- Format `<type>: <description>`; prefixes `feat|fix|refactor|docs|test|build`. Never `chore`/`style`/`ci`. Use `build:` for tooling/CI. PR-merged commits append `(#NN)`.
- **One commit per remediation batch.** Each `cy-fix-reviews` round produces exactly one local commit.
- Run `make verify` **once** before a commit batch. Don't re-run after committing — the pre-commit hook only runs `lint-staged` (husky's `core.hooksPath` bypasses skeeper's managed hooks; sidecar sync is manual via `skeeper sync`), so a passing pre-commit verify stays valid. Re-run only if the hook modified tracked source.
- If a pre-commit hook fails, do **not** `git commit --amend` — fix the issue and create a new commit.

## Code Search Hierarchy

1. **Grep / Glob** — local project code.
2. **`context7` skill** — external library docs.
3. **`exa-web-search-free`** — web research, news, external examples.

## Surface Map

Repo layout — **open the surface's instructions file before working in it**:

| Path            | Stack                                                               | Instructions              |
| --------------- | ------------------------------------------------------------------- | ------------------------- |
| `cmd/agh`       | Go binary entry point                                               | `internal/CLAUDE.md`      |
| `internal/`     | Go runtime daemon (ACP, SQLite, autonomy kernel, HTTP/UDS, network) | `internal/CLAUDE.md`      |
| `web/`          | React 19 SPA (Vite, TanStack, Tailwind, shadcn)                     | `web/CLAUDE.md`           |
| `packages/site` | Fumadocs documentation site (Bun)                                   | `packages/site/CLAUDE.md` |
| `packages/ui`   | Shared UI primitives (`@agh/ui`) for `web/` + `packages/site`       | `packages/ui/CLAUDE.md`   |

## Coding Style

Before editing any production `*.go` under `cmd/`/`internal/`, activate `agh-code-guidelines` (error wrapping `%w`, `errors.Is/As`, `slog`, `context` discipline, compile-time interface assertions, no hardcoded config). Hard invariants are in Critical Rules.

## Testing

- Activate `agh-test-conventions` before writing/editing any `*_test.go`; `consolidate-test-suites` before adding/moving a test (see the Critical Rules test-placement rule). Both skills carry their own detail.
- Non-negotiables: `t.Run("Should …")` subtests + `t.Parallel` default; status-code **and** body assertions; `-race`/`CGO_ENABLED=1`; integration/E2E build tags; runtime-contract co-ship (E2E mock + matchers ship with contract changes); 80% per-package coverage floor. `make verify` is the commit gate — test failures are production bugs.

### Schema Migrations

Any SQLite table/column/index/constraint change → activate `agh-schema-migration`; update the owning stream's declarative schema source (`schema.sql` or ordered domain fragments), append the next gap-free Goose SQL migration, refresh `atlas.sum` and sqlc output with `make codegen`, then pass `make codegen-check`. **Append-only identity:** existing migration bytes, versions, order, and `atlas.sum` history are immutable — never insert, rename, renumber, reorder, or edit an existing migration, weaken integrity checks, or hand-edit a Goose version table. Extend the canonical fresh/reopen/ahead/integrity/equivalence suites; boot-time `EnsureSchema` repair is forbidden.

## Memory & Lessons Learned

`docs/_memory/` is institutional memory — authoritative when CLAUDE.md is silent. Read the relevant file before the matching work:

- `standing_directives.md` — active engineering posture (SD-001..011); read before a TechSpec or architecture pivot.
- `spec-authoring-playbook.md` — mandatory preflight for `cy-create-prd`/`techspec`/`tasks` (enforced by `cy-spec-preflight`).
- `lessons/` (`L-001..031` + README) — durable lessons with confirmed root cause + evidence; scan the index by issue class.
- `glossary.md` — canonical vocabulary; read when naming anything or reviewing a rename.
- `_synthesis.md` + `analysis/` — evidence corpus behind the rules; read when challenging one.

Authoring: new lesson → `L-NNN-*.md` + update `lessons/README.md` (activate `lesson-learned`); new directive → next `SD-NNN` block. Lessons explain _why_ a rule exists — don't duplicate rules, don't add speculative warnings.

## Cross-References

- Backend/Go (architecture, autonomy, security, package layout): `internal/CLAUDE.md`. Web: `web/CLAUDE.md`. Site: `packages/site/CLAUDE.md`.
- Design tokens: `packages/ui/src/tokens.css` → generated spec `DESIGN.md`. Copy: `COPY.md`. Strategy/register/users/WCAG floor: `PRODUCT.md`.
- Institutional memory: `docs/_memory/`.
