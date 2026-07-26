# L-018 — Delegated docs lanes need a runtime-truth audit before acceptance

**Class:** Documentation / Spec authoring
**Date discovered:** 2026-05-05 (orch-improvs task 29 audit)
**Evidence sources:** Local audit on the Claude Opus docs delegation for the review-gate / cursor /
bundled-skills site narrative

## Context

`task_29` authored runtime narrative docs for the review gate, bundled orchestration skills, and
bridge notification cursors under `packages/site/content/runtime/core/autonomy/`. The work ran
through the mandatory frontend/docs delegation lane (`compozy exec --ide claude --model opus`).
The first finalization of the delegated pass produced confident-sounding prose that did not match
runtime truth.

The local audit caught:

- Invented review event types (`task.run_review_routed`, `task.run_review_circuit_opened`,
  `task.run_review_canceled`) that the daemon never emits. The canonical emit list lives in
  `internal/task/manager.go` review-event helpers and the consumer list in
  `web/src/systems/tasks/hooks/use-task-stream.ts:33-72`.
- A no-route review outcome misnamed `error`. The implemented outcome is `blocked`, persisted by
  `task.Service.RecordRunReview` and exposed through `agh task review`.
- A broken `/runtime/core/agent/context` link with no matching MDX route under `packages/site`.
- Bridge-notification UI placement claims that did not match the operator route in
  `web/src/routes/_app/tasks.$id.tsx` (Orchestration tab) or the run-detail route.
- Literal "No delivery yet" UI text the runtime never emits (the actual zero-state branch in
  `web/src/systems/tasks/components/tasks-bridge-notifications-card.tsx` shows cursor-identity
  fields, not that string).
- Public cursor-reset wording that exposed an unimplemented CLI/API verb. The cursor reset path is
  internal-only today (see `packages/site/content/runtime/core/autonomy/notification-cursors.mdx`
  notes that there is no public reset verb).

`packages/site/lib/runtime-autonomy-docs.test.ts` was extended in the same task with positive
substring assertions that pin the truthful phrasings, and a static-route metadata test was split
into a lightweight module so the docs Vitest run does not pull entire blog/changelog page trees
during metadata checks.

## Root cause

Delegated docs runs operate from a snapshot of intent. They are unusually good at producing prose
that _sounds_ like AGH. They do not have a self-audit step that proves event names, routes,
outcomes, and UI strings against the runtime, contracts, and live route components. When the
delegation finishes and reports success, the parent agent inherits the false confidence unless an
audit pass goes through the docs against authoritative sources before merge.

The same risk shows up if the delegated agent does not know to keep generated CLI/API references
read-only or to stop short of inventing CLI verbs that the daemon does not register.

## Rule

> Every delegated docs run, regardless of model class, must be audited against runtime truth
> before the docs branch closes. The audit checks generated CLI/API references, OpenAPI types,
> internal event-type lists, route-component selectors and copy, and config keys. Truthful
> phrasings become positive assertions in `packages/site/lib/runtime-autonomy-docs.test.ts` (or the
> equivalent docs checklist test) and the docs build is required to enforce them.

## Operationalization

When delegating docs work:

1. Before delegation, list the runtime sources of truth the delegated agent must respect: emit
   sites for events, transport handler files for routes, OpenAPI output, CLI cobra command tree,
   web route file, and `config.toml` reference. Pass them in the prompt.
2. After the delegated pass returns, audit the diff line-by-line against those sources. Treat any
   event name, route, outcome, status, CLI verb, config key, or UI literal that is not grep-able
   in the runtime as suspect.
3. Encode the truthful set as positive checklist assertions in the docs Vitest. Add forbidden
   phrasings only when they have appeared and been corrected — keep the test honest about real
   audit findings.
4. Re-run `cd packages/site && bun run source:generate && bun run content:generate &&
bun run typecheck && bun run test && bun run build` before claiming docs work complete.

## Anti-pattern

- Treating a delegated docs run's "PASS" as evidence the prose is correct.
- Hand-editing generated CLI/API reference pages to match the prose.
- Inventing CLI verbs, routes, or event names because they "would make sense".
- Letting page metadata or layout tests silently time out while waiting on an unrelated route
  module to load.

## Source

- `.compozy/tasks/orch-improvs/memory/task_29.md` (audit findings, corrections, verification)
- `packages/site/content/runtime/core/autonomy/review-gate.mdx`
- `packages/site/content/runtime/core/autonomy/notification-cursors.mdx`
- `packages/site/lib/runtime-autonomy-docs.test.ts`
- `packages/site/lib/static-route-metadata.test.ts`
- `packages/site/app/blog/metadata.ts`, `packages/site/app/changelog/metadata.ts` (lightweight
  metadata modules to keep the docs test suite focused)
- `internal/api/core/sse.go:54-60` (canonical `Name = event.Type` emit path used as audit anchor)
