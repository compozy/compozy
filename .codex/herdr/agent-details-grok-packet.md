# Delegation contract: Agent Details remediation

Repository: `/Users/pedronauck/Dev/compozy/agh`

Worker role/model: designated `designer` execution worker, Cursor TUI using `cursor-grok-4.5-high` (Grok 4.5 High).

Execution mode: direct execution, not plan mode. Work hands-on until every non-backend remediation requirement is implemented and verified. Do not merely audit, propose, or stop after a partial slice.

## Objective

Resolve the production, frontend architecture, interaction, accessibility, responsive, shared-UI, test, QA-tracker, documentation, and visual-evidence problems in `docs/design/opendesign/AGENT-DETAILS-REMEDIATION-PLAN.md`. Revalidate the plan against the current worktree first, then implement the target state. Fix root causes, delete obsolete alpha topology/components/tests, and do not preserve compatibility paths.

The user explicitly says **do not create a new TechSpec or other spec artifacts for this remediation**. Treat that as an authorized override of AD-001/AD-003/AD-004/AD-019/AD-021/AD-025 and Workstreams 0/1 only insofar as they demand a new spec. Still preserve/capture the current named OpenDesign references and produce the complete Visual Contract Mode evidence required to prove the implementation. Do not modify the remediation plan to pretend spec work happened.

## Mandatory skill/instruction activation before edits

- Read root `AGENTS.md`/provided repository instructions, `/Users/pedronauck/.codex/RTK.md`, `web/CLAUDE.md`, and `packages/ui/CLAUDE.md` as applicable.
- Activate and follow `agh-design`, `ui-craft`, `impeccable`, `no-workarounds`, and `agh-ui-screenshot` before UI edits.
- This is the repository's required `designer` execution pass. Declare the scene, Product register, and dials before editing.
- Activate all additional web skills required by `web/CLAUDE.md` for touched domains: at minimum `react`, `tailwindcss`, `vercel-react-best-practices`; add `tanstack`, `agh-data-boundaries`, `app-renderer-systems`, `consolidate-test-suites`, `vitest`, `testing-boss`, `storybook-stories`, `shadcn`, and others when their triggers fire.
- Before any test edit, state the invariant, owning layer, and canonical existing suite. Do not create duplicate/CSS-literal/snapshot/file-existence tests.
- Use `rtk` for every shell command.

## Claimed work slice

You own all necessary non-backend edits for this remediation, primarily:

- `web/src/routes/_app/agents*` and relevant route tests/preloading;
- `web/src/systems/agent/**`, route stories/fixtures, and canonical agent tests;
- `packages/ui/src/**` only for genuine generic primitive gaps, with colocated stories/tests and public export updates;
- user-facing docs/official AGH skill/QA scenario files only when current public behavior actually changes and repository rules require them;
- durable visual contract evidence under an appropriate `.compozy/tasks/agent-details-remediation/evidence/visual/` root (or another canonical durable root required by the screenshot skill), including reference, implementation, side-by-side, diff, comparison JSON, and PASS `review.md` per row.

Do not edit production Go, OpenAPI/backend DTOs, schema/migrations, generated backend clients, CLI/UDS/native tools, daemon/store/network code, or backend tests. If frontend truth depends on missing/incorrect backend behavior, record the exact missing contract, files/endpoints/types inspected, expected semantics, workspace-scope requirement, and a minimal controller action packet; continue every independent frontend/evidence task.

Do not edit unrelated dirty files. Do not run destructive git commands. Do not commit.

## Required product outcome

Close all applicable issue-register items AD-002, AD-005 through AD-030 after the user's no-spec override, including at minimum:

- mounted agent detail shell with `/agents/:name/settings` as a deep-linkable modal overlay; Back/close/direct-link/missing-agent behavior correct;
- body `DetailHeader` separate from route top bar, truthful identity/status/validity/category/origin/runtime placement;
- Overview metrics exactly Active, Runtime, Failed, Last activity using truthful canonical data available to the web; never client-aggregate incomplete pages;
- Overview/Instructions/Configuration/Sessions lanes and main/aside hierarchy, with all loading/empty/error/long/invalid/responsive states;
- centered, bounded, one-scroll-owner settings dialog with Basics, Runtime, Instructions, Access, MCP servers, Danger zone, persistent header/footer, dirty/validation/save-failure/conflict/delete-confirmation flows, focus trap/return and safe Escape/Back behavior;
- shared Provider · Model · Reasoning selector with truthful availability/reasoning/capability behavior, used in detail/settings without duplicate local selectors; pending/conflict/failure UI must retain server truth;
- agent list row/card metadata typography separated by semantic role;
- canonical `@agh/ui` primitives/tokens, no raw palette/magic tuple/shadow clone, no god files over 500 production lines;
- responsive behavior and keyboard/WCAG 2.2 AA contracts;
- explicit delete targets: old full-page settings layout/topbar actions/sidebar rail/local chrome/topbar-only identity/substitute metrics/duplicate selectors/obsolete tests/stale evidence;
- QA tracker impact: reset/add content-addressed `untested` agent scenarios as required, without running scenario QA;
- docs and `skills/agh/` impact resolved only from actual public behavior; use required writing skills before editing skills/instruction files;
- full AGH Impact Audit evidence for native tools, extensibility/hooks/config lifecycle, workspace isolation, and official AGH skill. For backend-owned proof gaps, report them to controller rather than editing backend.

## Visual contract

The named references are normative:

- `docs/design/opendesign/agent-detail.html`
- `docs/design/opendesign/agent-settings.html`
- `docs/design/opendesign/agents-list.html`
- `docs/design/opendesign/provider-model-reasoning-selector.html`

Follow `agh-ui-screenshot` Visual Contract Mode before implementation. Record exact `git hash-object` identities. Build an explicit state/viewport matrix covering every state required by section 11 of the remediation plan. Capture matched reference/implementation pairs at identical dimensions, generate diagnostics, inspect every pair, fix all blocking structural/accessibility divergences, and validate every bundle. Runtime truth wins, but any authorized difference must cite the user's no-spec override or an existing higher-authority runtime/design contract. Implementation-only captures do not count.

## Verification

- During iteration, run only touched Turbo lanes from repo root, for example `bunx turbo run lint typecheck test --filter=./web` and `--filter=./packages/ui` when touched.
- Run relevant existing route/component/story/e2e checks necessary for the claimed invariants.
- Do not run final `make verify`; the controller owns the single final monorepo gate.
- Do not leave Storybook/dev/static/browser processes alive. Follow screenshot ownership and teardown rules exactly; report owned PID cleanup evidence.

## Reporting contract

When done, provide a concise but complete report in the TUI with:

1. requirements/AD items closed, waived by explicit user no-spec instruction, or blocked on backend;
2. files changed and important delete targets removed;
3. tests/gates run with exact results;
4. visual evidence root, contract IDs, PASS/FAIL counts, and inspected screenshots;
5. any backend action packet for the controller;
6. AGH Impact Audit and QA-tracker action;
7. uncertainty and remaining blockers, if any.

Stop conditions: authentication/model rejection, unexpected unrelated dirty edits in a claimed file, repeated command failure after root-cause investigation, or a need to edit backend/contract files. In those cases, report exact evidence and wait; do not downgrade the model or switch tools.
