# BUG-20261005-loop-wait-example-unknown-event: The documented wait event cannot be validated

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-04, author a durable wait before a handoff action
- **Scenarios:** LP-live-run-survives-extension-disable
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

The DSL reference's durable-wait example names approval.received, which is not in the
runtime hook catalog. Copying that event into a handoff Loop is rejected before a Run
can start. The validator correctly reports watch_events_kind_unknown; the documentation
example is stale. The existing early-rejection bug remains fixed.

## Reproduction

- **Charter:** CH-pinned-wait-extension-lifecycle · **Tour:** Interrupt Tour
- **Environment:** isolated real daemon, resume-editorial Profile, StudioOperations

1. Read the Durable wait control example in the public DSL reference.
2. Use its event kind in a valid handoff Loop and run the public loop validate command.
3. Read the active hook catalog through compozy hooks events.

**Expected:** the example uses a supported event and explains catalog membership.
**Actual:** validation returns valid=false with watch_events_kind_unknown for approval.received.

## Evidence

- packages/site/content/docs/loops/dsl-reference.mdx, Durable wait control.
- docs/qa/evidence/2026-10-02-untested/loops-pinned-wait-definition-validate.json
- docs/qa/evidence/2026-10-02-untested/loops-pinned-wait-hook-catalog.json

## Fix

- **Fix commit:** pending
- Use a catalog-backed task completion event in the example and link the authoritative
  event catalog. Preserve the existing runtime validator and explicit rejection behavior.
- **Regression evidence:** validate the exact revised YAML snippet through the public CLI;
  no prose/snapshot unit test is added.

## Verification

The exact revised YAML node was extracted with the site's YAML dependency, placed in a
complete Loop and accepted by the public validator. The hook catalog independently
includes task.run.completed. The production validator remains unchanged; no prose or
snapshot test was added.

Evidence: loops-wait-doc-example-extracted.json and loops-wait-doc-example-validated.json
under docs/qa/evidence/2026-10-02-untested. Required site delivery checks and commit closure
remain pending.
