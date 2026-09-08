# CompozyOS

CompozyOS is a Go daemon managing ACP agents over JSON-RPC/stdio, SQLite state, HTTP/SSE, UDS, and a web UI. Capabilities must be extensible and manageable by agents through structured CLI/HTTP/UDS surfaces.

## Compatibility Policy

Real users run every release. Preserve the three regimes from SD-013 and L-040:

- **User state:** SQLite, `config.toml`, workspace files, and persisted layouts upgrade losslessly; every shape change ships its migration. Data loss requires the user's recorded ADR sign-off and a release-note migration block.
- **Public surfaces:** CLI verbs/flags/output, HTTP/UDS routes and DTOs, hooks, extension/bridge SDKs, config keys, and `compozy__*` IDs auto-migrate losslessly where possible; otherwise retain the old shape for one release after the replacement ships, warn with the replacement, then delete. Only surfaces documented `experimental` may break without that window.
- **Internal code:** Go packages, `web/`, `@compozy/ui`, specs, RFCs, and `.compozy/tasks/*` rename every consumer together; delete obsolete code without aliases or legacy branches.
- Compatibility translation belongs in boundary loaders/decoders/alias tables/migration SQL, with one shim generation and a named removal release. Breaking-change specs list delete targets and the regime of each. Read `docs/_memory/standing_directives.md` §SD-013 when designing a compatibility change; its planned gates are not implemented guarantees.

## Working Rules

- Preserve unrelated work. Destructive Git (`restore`, `checkout`, `reset`, `clean`, `rm`, `stash`) requires explicit user permission.
- Conversation in Brazilian Portuguese; code, docs, specs, commits, and memory artifacts in English.
- Use `go get` for Go dependencies and `bun add` for JS dependencies; generated files change through their owning generator.
- Keep production source files cohesive and at most 500 lines; extract before growing an oversized file. Handle errors or justify an intentional discard.
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

Load a skill when the user names it or its procedure resolves a task-specific need. Start with the owning skill; add another only for a distinct unresolved concern. Installed skills, generic language knowledge, and task completion do not automatically create extra gates. Read only the applicable reference sections; reuse previously read material while it remains current.

- Go conventions: `eng-code-guidelines`; Go tests: `eng-test-conventions`. Use `eng-consolidate-test-suites` when placement is unclear, and language/debugging references when the problem needs them.
- SQLite changes: `eng-schema-migration`; wire changes: `eng-contract-codegen-coship`; authorization/pagination/cache boundaries: `eng-data-boundaries`.
- Specs/tasks: use the requested `cy-*` entry point and its applicable branches. Reuse approved research; peer review remains opt-in after saving the approved spec. Apply only user-selected peer-review findings.
- Design-system/redesign: `eng-design`; visual-reference comparison: `eng-ui-screenshot`. Use additional design skills for the dimension being changed, and delegate design work when a suitable agent is available and useful.
- Agent instruction edits: `writing-agents-md`; skill edits: `writing-skills`; incident lessons: `lesson-learned`.
- Resolve bundled skill helpers from the skill's actual root, such as `.agents/skills/<skill>/scripts/`; never depend on an ambiguous working directory.

## Verification and Delivery

Use the cheapest check that can expose the changed behavior, plus the required delivery gates. Evidence remains valid for the inputs it checked; a new message or commit alone does not invalidate it. Repeat or broaden only after relevant changes, failures, or unresolved risk.

- Before commit/push, `make gate` must pass with zero warnings/errors; docs-only changes that cannot affect test/lint/typecheck are exempt. The wrapper selects affected lanes and caches evidence under `.cache/gate/`; use `make gate-status` for current records. Missing merge bases or unclassified paths require diagnosis, not a fallback full run.
- PR delivery requires every required CI check green for the current head. Diagnose finished failures while other jobs run. A draft PR with pending/red checks remains in progress. `make gate-full` is opt-in local diagnostics; local full/E2E runs share a machine lock, so do not kill a queued run.
- Frontend validation runs through Turborepo from the repo root, with affected-package filters during iteration. Package-local test/typecheck runs are not delivery evidence.
- Review the changed diff once before final validation; repeat review only for subsequent relevant changes. Report the result, commands/evidence, and remaining limits concisely; no fixed report template for routine tasks.
- Commits: `<type>: <description>` with `feat|fix|refactor|perf|docs|test|build|ci`, no scopes. A failed pre-commit hook is repaired in a new commit, not an amend. `cy-fix-reviews` keeps one local commit per remediation batch.

## Compozy Cross-Surface Impact Audit

For behavior/contract changes, record the affected native tools, extensibility/hooks/config, workspace data isolation, and official skill (`skills/compozy/`), including Web/Docs impact. Use `docs/_memory/change-impact.md` once at the owning spec/task/PR; downstream tasks cite and update it rather than restating it. Editorial instruction changes may state `not applicable — editorial only`.

## QA

Changed user-visible behavior updates the affected `docs/qa/scenarios/` files and verifies those scenarios before delivery. Reuse current evidence for unchanged behavior. Spec loops collect slice evidence and run the trailing QA pair once for remaining integration journeys; intermediate tasks flag affected scenarios. Do not launch a full lab for editorial changes or repeat completed walks without a relevant change.

For release/scenario labs, use `eng-qa-bootstrap` and the applicable `qa-report`/`qa-execution` branches. Parallel labs require unique homes, ports, and sockets; derive proxy/provider settings from the manifest and serialize config writes per home. Register lab processes, run the manifest teardown command or `make qa-reap` on every exit, and retain `teardown.json` with `clean: true`. Failed walks require repair and re-walk; only documented external/decision blockers may remain unverified.

## Design, Copy, and Memory

Reuse `@compozy/ui` exports from `packages/ui/src/index.ts`. `packages/ui/src/tokens.css` and generated `DESIGN.md` own visual values; regenerate tokens with `make codegen`, never edit generated regions. Runtime truth owns content and controls; named references own visual language. Read `COPY.md` for product-facing text and `docs/_memory/glossary.md` for domain naming. Add supporting UI copy only when requested or needed to prevent error.

Read the matching entry in `docs/_memory/lessons/README.md` for an incident class, and applicable standing directives for design decisions. Lessons explain evidence and scoped constraints; historical process descriptions do not add new mandatory stages. Spec authoring uses `docs/_memory/spec-authoring-playbook.md`. Load synthesis/analysis only when investigating the rationale behind a rule.
