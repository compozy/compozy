---
name: agh-test-conventions
description: >-
  Go test-shape discipline for AGH. Use when writing or editing *_test.go under
  cmd or internal after test placement is justified. Do not use for non-Go
  tests, fixture-only changes, or as a replacement for consolidate-test-suites.
trigger: implicit
---

# AGH Test Conventions

Apply Go-specific shape rules only after `consolidate-test-suites` identifies
the invariant, owning layer, and canonical suite. This file owns the application
order; its reference owns the rules.

## Procedures

**Step 1: Confirm Placement**

1. Use `consolidate-test-suites` before creating a new Go test file, moving coverage, or adding tests primarily for a task checklist or coverage target.
2. Record the invariant, owning layer, and canonical suite before applying Go-specific shape rules.
3. If no invariant or owning layer exists, stop. Do not add a Go test just to raise coverage.

*Done when:* one durable invariant, one owning layer, and one canonical suite are recorded for every changed test.

**Step 2: Load the Canonical Shape**

1. Determine whether the edit creates a new test file, adds cases to an existing test, or refactors an existing test.
2. Read the existing canonical suite and `.agents/skills/agh/agh-test-conventions/references/test-shape-rules.md` in full.
3. Apply every matching rule for subtests, parallelism, errors, assertions, interfaces, build tags, integration/E2E behavior, mocks, coverage, helpers, and race/cgo.

*Done when:* each changed case conforms to every matching reference section and no duplicate invariant was added.

**Step 3: Preserve the Runtime Contract**

1. Co-ship ACP/E2E fixtures, typed matchers, generated contracts, and cross-surface expectations when runtime behavior changes.
2. Use real SQLite, subprocess mocks, and other production-like boundaries where the owning layer requires them.
3. Treat a failing assertion as evidence about production behavior; repair production code unless the test is proven invalid against the contract.

*Done when:* the test exercises the real owner closely enough to fail on the named regression and all changed runtime-contract fixtures agree.

**Step 4: Validate the Changed Suite**

1. Run the read-only checker with its repo-root path:
   `python3 .agents/skills/agh/agh-test-conventions/scripts/check-test-conventions.py <file_path>`
2. Fix real findings; document a proven heuristic false positive without weakening the canonical rules.
3. Run `go test -race ./<owning-package>/...` for the affected package with `CGO_ENABLED=1`, then the required scoped lint lane.
4. Reserve the single full `make verify` for the task completion gate after source freeze.

*Done when:* the checker and scoped race/lint lanes are green and exactly one fresh full gate is scheduled or complete for the finished task.

## Error Handling

- **Existing file uses non-`Should` naming throughout:** refactor the touched test function into canonical subtests; do not add another legacy-shaped case or rewrite unrelated suites.
- **The convention checker returns a false positive:** prove the syntax is valid against the canonical reference and record the narrow exception; never use the heuristic to waive a real rule.
- **`t.Setenv` used inside a helper that callers cannot inspect:** read the helper transitively. If env mutation occurs anywhere in the call graph, the entire test stays serial.
- **Race-enabled tests touching cgo:** use the repository's race-enabled command path, which forces `CGO_ENABLED=1`; do not trust ambient env.
