---
name: consolidate-test-suites
description: Use when adding, moving, reviewing, or auditing tests in AGH to identify the invariant, owning layer, and canonical suite before changing coverage. Do not use as a replacement for framework-specific testing skills or final verification gates.
---

# Consolidate Test Suites

## Procedures

**Step 1: Name The Invariant**
1. Identify the rule that must stay true after the change.
2. Write the invariant in one sentence before editing any test.
3. If no durable behavior, contract, security boundary, concurrency rule, data invariant, or public API promise can be named, stop and report that no new automated test is justified.

**Step 2: Choose The Owning Layer**
1. Select exactly one primary owning layer: unit, integration, end-to-end, static analysis, codegen, visual QA, documentation build, or manual QA evidence.
2. Prefer the lowest layer that can prove the invariant against the real owner.
3. Do not duplicate the same invariant in multiple layers unless each layer proves a distinct failure mode.
4. Read `.agents/skills/agh/consolidate-test-suites/references/test-placement-rules.md` when the owning layer is not obvious.

**Step 3: Reuse The Canonical Suite**
1. Search existing tests before creating files: use `rg --files` plus focused `rg` for the feature, public API, component, route, command, or package name.
2. Prefer adding or adjusting a case in the existing canonical suite for the owning layer.
3. Create a new standalone regression test only when no existing suite owns the invariant.
4. If a new test file is created, record why an existing suite could not own the case.

**Step 4: Reject Low-Signal Tests**
1. Do not add tests that merely freeze implementation details, CSS literal values, generated output, snapshots, prose strings, config shape, or file existence.
2. Allow static artifact tests only when the artifact itself is the product contract and no stronger gate exists.
3. Reject Storybook setup/config/bootstrap, CI workflow/action YAML, Mage/Make/package-script plumbing, generic config/file existence, generated-output drift, snapshots/goldens, and coverage-padding unless the artifact is itself the product/operator contract.
4. For UI/design changes, prefer lint rules, Storybook visual verification, accessibility checks, or behavior-level component tests over literal token/prose snapshots.
5. For bug fixes, add the narrowest regression coverage at the owning layer after reproducing the bug.

**Step 5: Record The Decision**
1. In task notes, completion notes, or the final response, report:
   - Invariant.
   - Owning layer.
   - Canonical suite updated, or no-new-test rationale.
   - Verification command.
2. If no new automated test is added, cite the existing suite or gate that already proves the invariant.

## Error Handling

- **Existing test appears redundant:** identify the invariant it claims to own. If another suite already proves the same failure mode, consolidate into the owner and remove the duplicate only when the task explicitly includes test cleanup.
- **Coverage target conflicts with test placement:** do not add filler tests to raise coverage. Report the coverage gap and add behavior-bearing tests only where a named invariant exists.
- **Review requests a new regression test without an invariant:** ask for or infer the invariant from the bug. If none exists, mark the test request unjustified with evidence.
- **A task template says tests are required:** interpret that as a required test decision, not automatic creation of unit and integration tests.
