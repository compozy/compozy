# BUG-20260818-runtime-input-split-controls: Runtime input replaced the canonical selector on catalog error

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea
- **Journey Step:** J-01, fill the declared runtime input
- **Scenarios:** LP-select-typed-loop-entities
- **Found:** 2026-08-18 · **Report:** docs/qa/reports/2026-08-18-typed-loop-inputs.md

## Summary

The Loop run form replaced the shared `RuntimeSelector` with three raw text inputs when its runtime
catalog failed. The Loop Storybook story used a missing workspace fixture and exposed that branch,
including an unrelated `Workspace not found: ws_default` message.

## Fix

- **Root cause:** `RuntimeValueControl` had a catalog-error-only replacement UI, and the story used
  a workspace ID absent from the canonical Storybook workspace handlers.
- **Correction:** Runtime inputs always render `RuntimeSelector`; catalog status stays inside that
  component. The primary story now uses the canonical workspace fixture, and an error story pins
  the same selector under catalog failure.
- **Fix commit:** pending implementation commit
- **Regression coverage:** `RuntimeCatalogUnavailable` in
  `web/src/systems/loops/components/stories/loop-run-form.stories.tsx`.

## Verification

- Focused Loop input tests, Web typecheck, and the Storybook build pass.
- The built `ImplementTasks` story renders one `Runtime: Codex / gpt-5.6` button. The unavailable
  story keeps that button and reports the missing workspace inside its popup.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-typed-loop-inputs-20260819-015537-040869-lab/qa-artifacts/qa/typed-runtime-selector-story.png`.
