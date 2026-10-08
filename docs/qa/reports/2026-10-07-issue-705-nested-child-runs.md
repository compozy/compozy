# Issue 705: nested child runs on the parent run page

## Finding

On `f0036344c`, a parent step parked on its child run (`awaiting_child`) drew the
same dashed ring as an unreached `pending` step, and nothing on the parent said
which step any child was on, how far along it was or for how long. The roster
already served `child_loop_run_id` on every `awaiting_child` row, so the Web
could read each child without a new route.

## Walk

Targeted lab `issue-705-nested-loops-20261008-013430-865926` (surfaces web, cli,
runtime), daemon built from the branch, no agents or providers:

- Child `fix-one-batch`: transform → durable `wait` (`for: 40m`) → transform,
  `concurrency: allow`.
- Parent `nested-wave`: transform → `fan-out` of three items → `run-loop`
  (`mode: await`) → transform.

All three branches reached `awaiting_child` with a child id; one child's wait was
paused through `compozy loop node pause`. The same daemon served two Web
builds: `main` reproduced the issue (identical rings, no child visibility), and
the branch showed the delegated glyph plus a collapsed "3 child runs" that opened
three rows with status, current step and park reason, step count and a ticking
elapsed clock.

The walk found one defect before delivery: `GET /loop-runs/:id` serves a zeroed
`progress`, so a count built from it read nothing. Step counts now come from the
child's briefing, which owns them, and the Storybook fixture serves the detail
zeroed the same way so the component test proves the route choice. It also
showed three rows reading "fix batch" for one fanned node; such branches now carry
their item slot.

## Verification

- Strict lab auditor: PASS, zero blockers or warnings.
- Targeted local checks: `oxfmt --check`, `oxlint --deny-warnings` on the changed
  web and UI surfaces, `typecheck` for web and `@compozy/ui`, the full loops Vitest
  system plus the design-system showcase (850 tests), and the StateGlyph suite
  (16 tests).
- Lab teardown: clean.

The full `make gate` and E2E lanes are delegated to the PR's required CI checks at
the user's request. Scenario: `LP-web-run-nested-child-runs`.

## Impact

Web only. No schema, migration, CLI, HTTP/UDS, config, hook or native-tool
change. `@compozy/ui` gains the `delegated` StateGlyph state.
