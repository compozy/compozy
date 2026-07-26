# BUG-20260713-loop-contract-goal-not-editable: A custom Loop's contract goal cannot be changed in the UI

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-06 publish and run the same custom Loop with and without a goal
- **Scenarios:** LP-toggle-loop-goal
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md

## Summary

The workspace Loop builder can add and configure a graph `goal` action, but it exposes no editor for the Loop contract's `goal` or `definition_of_done`. The DSL tab is explicitly read-only and Configure says structural goal changes belong in Fork & edit. Bruno therefore cannot use the UI to publish and run one custom Loop first with a goal and then without the optional goal.

## Reproduction

1. Fork and publish `reviews-watch` as a workspace Loop.
2. Open its builder and inspect Graph, DSL, toolbar, palette, and inspector surfaces.
3. Open Configure and inspect the structural-boundary explanation.
4. Try to clear or replace the contract goal and definition of done.

**Expected:** The writable builder exposes contract goal and definition-of-done authoring, including an intentional way to clear an optional goal, validates it with the shared linter, and publishes it under CAS.
**Actual:** Only graph nodes are editable; the contract is visible only in read-only DSL/detail/run projections.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-007-loop-fork-publish-v1-fixed.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-contract-goalless-published.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-goalless-published-v1.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-goalless-v1-real-run.dom.txt`
- Browser replay configured a graph Goal node until only `unreachable_node` remained, proving that graph Goal authoring is distinct from the missing contract editor.

## Fix

- **Root cause:** The writable builder view-model only exposed graph node fields. Contract `goal` and `definition_of_done` stayed in the definition payload but had no authored controls, while the DSL remained intentionally read-only.
- **Fix:** Add a dedicated Contract rail to the existing Loop editor, keep the optional goal and required definition of done in the canonical definition draft, and publish the full contract through the existing expected-version compare-and-swap path. Workspace-owned detail and Configure now use `Edit`; read-only sources retain `Fork & edit`.
- **Fix commit:** pending final task commit
- **Regression test:** Canonical `loop-editor.test.tsx` coverage owns contract editing, optional-goal clearing, definition-of-done persistence, and publish payload preservation. The worker's focused Loop lane passed 49/49 tests; scoped format/lint and React Doctor 100/100 also passed.

## Verification

- Same-persona in-app-browser replay passed on 2026-07-13. The goal-bearing workspace v1 had already started as `looprun-7e6dbcacdf292853`. Bruno then cleared the goal through the Contract rail, changed the definition of done, published a goal-less version, and confirmed the fresh Run projection contained only the saved definition of done.
- A second strict replay recreated the read-only `reviews-watch` shadow, cleared its goal, published workspace v1, and started real run `looprun-c0e322b615e43c12`. The run detail rendered no goal and preserved the definition of done. The run was stopped through the UI before cleanup.
