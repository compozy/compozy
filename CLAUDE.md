# CompozyOS

CompozyOS is a Go daemon managing ACP agents over JSON-RPC/stdio, SQLite state, HTTP/SSE, UDS, and a web UI. Capabilities must be extensible and manageable by agents through structured CLI/HTTP/UDS surfaces.

## Compatibility Policy

Real users run every release. Preserve the three regimes from SD-013 and L-040:

- **User state:** SQLite, `config.toml`, workspace files, and persisted layouts upgrade losslessly; every shape change ships its migration. Data loss requires the user's recorded ADR sign-off and a release-note migration block.
- **Public surfaces:** CLI verbs/flags/output, HTTP/UDS routes and DTOs, hooks, extension SDKs, config keys, and `compozy__*` IDs auto-migrate losslessly where possible; otherwise retain the old shape for one release after the replacement ships, warn with the replacement, then delete. Only surfaces documented `experimental` may break without that window.
- **Internal code:** Go packages, `web/`, `@compozy/ui`, specs, RFCs, and `.compozy/tasks/*` rename every consumer together; delete obsolete code without aliases or legacy branches.
- Compatibility translation belongs in boundary loaders/decoders/alias tables/migration SQL, with one shim generation and a named removal release. Breaking-change specs list delete targets and the regime of each. Read `docs/_memory/standing_directives.md` §SD-013 when designing a compatibility change; its planned gates are not implemented guarantees.

## Working Rules

- Preserve unrelated work. Destructive Git (`restore`, `checkout`, `reset`, `clean`, `rm`, `stash`) requires explicit user permission.
- Conversation in Brazilian Portuguese; code, docs, specs, commits, GitHub issues, and memory artifacts in English.
- All file changes happen in a worktree and the root checkout stays clean. Gitignored runtime files such as spec directories under `.compozy/tasks` and `.compozy/work-log.md` live in the workspace root.
- Use `go get` for Go dependencies and `bun add` for JS dependencies; generated files change through their owning generator.
- Keep production source files cohesive and at most 500 lines; extract before growing an oversized file.
- Before changing a test, identify its invariant, owning layer, and existing suite. Reuse that suite and suitable utilities. Add coverage for observable behavior; prose/CSS/config/snapshot assertions need an actual artifact contract without a stronger owning check. Fix production regressions without weakening tests.
- Follow the user's authorized scope. Reuse current research, decisions, and verification; ask only when an unresolved decision blocks correct work. Avoid unrelated cleanup.

## Surface Instructions and Skills

Read the relevant subtree instructions before working there:

| Surface                    | Instructions              |
| -------------------------- | ------------------------- |
| `cmd/compozy`, `internal/` | `internal/CLAUDE.md`      |
| `web/`                     | `web/CLAUDE.md`           |
| `packages/site`            | `packages/site/CLAUDE.md` |
| `packages/ui`              | `packages/ui/CLAUDE.md`   |

Load a skill when the user names it or its procedure resolves a task-specific need; start with the owning skill.

- Go conventions: `eng-code-guidelines`; Go tests: `eng-test-conventions`. Use `eng-consolidate-test-suites` when placement is unclear, and language/debugging references when the problem needs them.
- SQLite changes: `eng-schema-migration`; wire changes: `eng-contract-codegen-coship`; authorization/pagination/cache boundaries: `eng-data-boundaries`.
- Dispatching or supervising sessions, filing an issue, opening or updating a PR, handling review feedback or CI, or declaring work done: `eng-delivery`.
- Specs: `cy-create-spec` owns authoring, `_slices.md`, and opt-in peer review; `docs/_memory/spec-authoring-playbook.md` owns Compozy compatibility, contract, and lesson rules. Reuse approved research.
- Design-system/redesign: `eng-design`; visual-reference comparison: `eng-ui-screenshot`. Use additional design skills for the dimension being changed, and delegate design work when a suitable agent is available and useful.
- Agent instruction edits: `writing-agents-md`; skill edits: `writing-skills`; incident lessons: `lesson-learned`.
- Resolve bundled skill helpers from the skill's actual root, such as `.agents/skills/<skill>/scripts/`; never depend on an ambiguous working directory.

## Verification and Delivery

Use the cheapest check that can expose the changed behavior. CI owns the delivery gates.

- `make gate`, `make gate-full`, repository-wide lint, full test or `-race` suites, and E2E run only in CI, unless the operator asks for that local run; concurrent local runs saturate the machine. Before commit/push, run focused tests and focused lint for what you changed, with zero warnings/errors.
- PR delivery requires the CI checks named in `eng-delivery` green for the current head. Diagnose finished failures while other jobs run. A draft PR with pending/red checks remains in progress. Local full/E2E runs share a machine lock, so do not kill a queued run.
- Frontend validation runs through Turborepo from the repo root, with affected-package filters during iteration. Package-local test/typecheck runs are not delivery evidence.
- Commits: `<type>: <description>` with `feat|fix|refactor|perf|docs|test|build|ci`, no scopes. A failed pre-commit hook is repaired in a new commit, not an amend.

## Compozy Cross-Surface Impact Audit

For behavior/contract changes, record the affected native tools, extensibility/hooks/config, workspace data isolation, and official skill (`skills/compozy/`), including Web/Docs impact. Use `docs/_memory/change-impact.md` once at the owning spec/task/PR; downstream tasks cite and update it rather than restating it. Editorial instruction changes may state `not applicable — editorial only`.

## QA

Changed user-visible behavior updates the affected `docs/qa/scenarios/` files and verifies those scenarios before delivery. Spec delivery collects slice evidence and verifies remaining integration journeys once, after integration; slices flag affected scenarios.

For release/scenario labs, use `eng-qa-bootstrap` and the applicable `qa-report`/`qa-execution` branches; the skill owns manifest, teardown, and `make qa-reap` semantics. Parallel labs require unique homes, ports, and sockets. Failed walks require repair and re-walk; only documented external/decision blockers may remain unverified.

## Design, Copy, and Memory

- **Compose `@compozy/ui` before writing a component.** The inventory is `packages/ui/src/index.ts` plus the `primitives.ts` and `exports/*.ts` files it re-exports. Decide in order: compose an existing primitive; a domain-prefixed variant only for behavior the primitive cannot express, since a prefixed or restyled copy is still a duplicate; a new generic primitive lands in `packages/ui` with story and test, migrating every callsite in the same change. `compozy-ui-reuse/no-shadow-ui-primitive` catches same-name shadows only. Rationale: L-031 in `docs/_memory/lessons/`.
- `packages/ui/src/tokens.css` and generated `DESIGN.md` own visual values; regenerate tokens with `make codegen`, never edit generated regions. Runtime truth owns content and controls; named references own visual language.
- Read `COPY.md` for product-facing text and `docs/_memory/glossary.md` for domain naming. Add supporting UI copy only when requested or needed to prevent error.

Read the matching entry in `docs/_memory/lessons/README.md` for an incident class, and applicable standing directives for design decisions. Lessons explain evidence and scoped constraints; historical process descriptions do not add new mandatory stages. Spec authoring uses `docs/_memory/spec-authoring-playbook.md`.
