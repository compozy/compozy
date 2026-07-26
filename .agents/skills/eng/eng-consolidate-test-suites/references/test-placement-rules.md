# Test Placement Rules

## Definitions

- Invariant: the rule that must stay true.
- Owning layer: the lowest layer that truly owns and can prove that rule.
- Canonical suite: the existing suite for that owning layer.

Default: reuse an existing canonical suite. Do not create a new standalone regression test unless the exception rule below allows it.

## Hard Rules

- Identify the invariant before adding or moving any test.
- Identify one primary owning layer: unit, integration, end-to-end, static analysis, codegen, visual QA, documentation build, or manual QA evidence.
- First try to place coverage in an existing canonical suite for that layer.
- Prefer editing an existing test file over creating a new test file.
- Do not add the same invariant in multiple layers unless each layer covers a different failure mode. Name the distinct failure mode for each.
- Do not add tests that lock in implementation details unless that implementation unit itself owns the invariant.
- Do not create a standalone regression test because it is faster or easier.
- If the invariant and owning layer cannot be named, stop and report that placement is not justified.

## Decision Order

1. Reproduce or identify the behavior change.
2. Name the invariant.
3. Search for existing suites that already cover that invariant or the owning API.
4. Choose the lowest layer that proves the invariant with the least mocking and the strongest ownership.
5. Update the canonical suite.
6. Create a new test file only if no canonical suite exists, and document why.
7. Run the narrow verification command plus the repository gate required by the parent instructions.

## Layer Guidance

- Unit: pure functions, local validation, deterministic formatting, typed state transitions, small adapters with no external process boundary.
- Integration: persistence, API/CLI/UDS boundaries, codegen consumers, filesystem interactions, provider contracts, cross-package behavior.
- End-to-end: user journeys, daemon/web/browser flows, process lifecycle, multi-agent/runtime behavior, race-prone workflows.
- Static analysis: banned syntax, class patterns, token usage, imports, generated drift, naming rules.
- Codegen: generated OpenAPI, CLI references, typed SDK artifacts.
- Visual QA: spacing, layout, color, interaction appearance, responsive behavior, Storybook capture diffs.
- Documentation build: generated docs, frontmatter schema, broken links, route metadata, source generation.
- Manual QA evidence: one-off UX judgment, release scenario evidence, or cases where automation would only duplicate visual/human review.

## Common Rejections

- A CSS token test that repeats many literal values from `tokens.css` without proving a public API failure mode.
- A docs test that asserts arbitrary prose substrings instead of generated source validity or broken-link behavior.
- A snapshot whose only purpose is to freeze current markup shape.
- A component test that asserts private class names when the public behavior is role, state, text, or interaction.
- A route test duplicated at hook, component, and browser layers without distinct failure modes.

## Allowed Standalone Test Exceptions

- A new public API or package has no existing suite.
- The regression belongs to a new boundary that no current suite exercises.
- The bug was caused by missing canonical ownership, and the new test file becomes that owner.
- A security or data-loss invariant needs an isolated corpus that would make existing suites noisy or fragile.
- The repository has an established pattern for a file-level contract suite, and the new file follows that pattern with narrowly scoped assertions.
