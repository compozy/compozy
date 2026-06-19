# Patterns — Twelve Cross-Framework Principles for Writing Tests That Survive Refactor

## Contents

- [How to use this catalog](#how-to-use-this-catalog)
- [Pattern 1 — Query by behavior, never by internals](#pattern-1--query-by-behavior-never-by-internals)
- [Pattern 2 — Selector hierarchy: role → label → text → test-id → structural](#pattern-2--selector-hierarchy-role--label--text--test-id--structural)
- [Pattern 3 — Wait on observable conditions, never on the clock](#pattern-3--wait-on-observable-conditions-never-on-the-clock)
- [Pattern 4 — Each test is independent and order-free](#pattern-4--each-test-is-independent-and-order-free)
- [Pattern 5 — Set up state before tests, do not clean up after](#pattern-5--set-up-state-before-tests-do-not-clean-up-after)
- [Pattern 6 — Test observable behavior, not implementation](#pattern-6--test-observable-behavior-not-implementation)
- [Pattern 7 — One behavior per test (not one assertion)](#pattern-7--one-behavior-per-test-not-one-assertion)
- [Pattern 8 — Test names read as specifications](#pattern-8--test-names-read-as-specifications)
- [Pattern 9 — Table-driven / parameterized for input variation](#pattern-9--table-driven--parameterized-for-input-variation)
- [Pattern 10 — Build test data via factories and builders](#pattern-10--build-test-data-via-factories-and-builders)
- [Pattern 11 — Mock at boundaries you do not control](#pattern-11--mock-at-boundaries-you-do-not-control)
- [Pattern 12 — Page Object Model is a tool, not a religion](#pattern-12--page-object-model-is-a-tool-not-a-religion)
- [Test structure shapes](#test-structure-shapes)
- [Test data tradeoffs](#test-data-tradeoffs)
- [Common framework gotchas](#common-framework-gotchas)
- [Sources](#sources)

## How to use this catalog

Each pattern names a principle, gives cross-framework evidence (Playwright, Testing Library, Cypress, Jest, pytest, Go), shows agnostic pseudo-code, and flags when to break it. Frameworks are evidence, not the subject — the principle is what transfers.

## Pattern 1 — Query by behavior, never by internals

**Principle.** Selectors that describe what a user perceives survive refactors of internals. Selectors tied to CSS classes, DOM indices, or generated IDs break on cosmetic change and couple tests to private structure.

**Evidence.**

- Playwright: "Locators are the central piece of Playwright's auto-waiting and retry-ability… resilient to changes in the DOM." (https://playwright.dev/docs/best-practices)
- Testing Library: priority list ranks `getByRole` first; falls back to label, text, test-id as last resort. (https://testing-library.com/docs/queries/about)
- Cypress: *"Don't target elements based on CSS attributes such as: `id`, `class`, `tag`."* Use `data-*` attributes as a stable test contract. (https://docs.cypress.io/app/core-concepts/best-practices)

**Pseudo-code.**

```
// good
click(query.byRole("button", { name: "Submit" }))
// bad
click(query.bySelector(".btn.btn-large.primary"))
```

**When to break it.** Structural elements with no accessible text (notification badges, counters, list items). Add a stable test-id and treat it as an explicit contract.

## Pattern 2 — Selector hierarchy: role → label → text → test-id → structural

**Principle.** Resilience and accessibility correlate. If the test can find an element via role and accessible name, real users and assistive tech can too. Each step down the ladder loses semantic grounding.

**Ladder (most → least preferred).**

1. ARIA role + accessible name
2. Label text (for form fields)
3. Visible text content
4. Alt text / title (image-only or fallback)
5. Explicit `data-testid` / `data-cy`
6. Structural selector (id / class / xpath) — escape hatch only

**Evidence.** Testing Library publishes this exact ordering (https://testing-library.com/docs/queries/about). Playwright and Cypress agree (https://playwright.dev/docs/best-practices, https://docs.cypress.io/app/core-concepts/best-practices).

**When to break it.** Only when the rung above is ambiguous, dynamic, or genuinely non-semantic. Step down one rung at a time; document why.

## Pattern 3 — Wait on observable conditions, never on the clock

**Principle.** Hard-coded sleeps overshoot on fast machines and underrun in CI. Condition-based waits self-tune and surface the real failure (`element never became visible`) instead of "test timed out".

**Evidence.**

- Playwright: *"Web-first assertions auto-retry."* `expect(locator).toBeVisible()` is correct; `expect(await locator.isVisible()).toBe(true)` is not. (https://playwright.dev/docs/best-practices)
- Testing Library: `findBy*` (Promise + retry) replaces `getBy*` when timing is involved. (https://testing-library.com/docs/queries/about)
- Cypress: avoid `cy.wait(timeout)`; use `cy.intercept().as('x'); cy.wait('@x')` to wait on a network event. (https://bugbug.io/blog/testing-frameworks/cypress-best-practices)
- Empirical data: *"developers thought they 'fixed' the flaky tests by increasing some time values… these time values actually have no effect"* (https://trunk.io/blog/the-ultimate-guide-to-flaky-tests)

**Pseudo-code.**

```
// good
await expect(query.byText("Welcome")).toBeVisible()
// bad
await sleep(2000)
assert(query.byText("Welcome").exists)
```

**When to break it.** Never for normal element waits. A short sleep can be defensible for throttling a polling load test — flag it explicitly.

## Pattern 4 — Each test is independent and order-free

**Principle.** Coupled tests cascade. One early failure masks downstream bugs; reordering or running one test in isolation breaks.

**Evidence.**

- Playwright: *"Each test should be completely isolated… with its own local storage, session storage, data, cookies."* (https://playwright.dev/docs/best-practices)
- Cypress: change `it` to `it.only` — if it still passes, the test is good; otherwise refactor into one larger test or use `beforeEach`. (https://docs.cypress.io/app/core-concepts/best-practices)
- Go: parallel subtests demand captured loop variables and zero shared state. (https://go.dev/wiki/TableDrivenTests)

**Operational rule.** A test that does not pass alone is a placement bug, not a feature.

## Pattern 5 — Set up state before tests, do not clean up after

**Principle.** "After" hooks aren't guaranteed to run (refresh, crash, abort). Before-hooks always do. State always starts known-good.

**Evidence.** Cypress: *"Clean up state before tests run… `afterEach` has no guarantee."* (https://docs.cypress.io/app/core-concepts/best-practices)

**Pseudo-code.**

```
// good
beforeEach(() => db.reset(); seed(); login());
// bad — runs only when the test exits cleanly
afterEach(() => db.reset());
```

**When to break it.** External resource leaks (sockets, temp files) must still be released via teardown. But *state for the next test* belongs in setup.

## Pattern 6 — Test observable behavior, not implementation

**Principle.** Refactoring should never break a passing test. If renaming a private function breaks tests, the test is reading internals.

**Evidence.**

- Playwright: *"Tests should typically only see / interact with the same rendered output the user sees."* (https://playwright.dev/docs/best-practices)
- React Testing Library: *"Behavior-driven, with a focus on not testing implementation details of a component."* (https://github.com/patternfly/patternfly-react/wiki/React-Testing-Library-Basics,-Best-Practices,-and-Guidelines)

**When to break it.** Pure-function unit tests legitimately assert on outputs. The rule applies at the component / integration layer.

## Pattern 7 — One behavior per test (not one assertion)

**Principle.** "One assertion per test" is a unit-test myth that misapplies to integration and E2E. The real rule: each test exercises one behavior; multiple assertions describing that behavior are encouraged.

**Evidence.** Cypress: *"Writing integration tests is not the same as unit tests… any single command could implicitly fail."* Chain multiple assertions in one test. (https://docs.cypress.io/app/core-concepts/best-practices)

**Pseudo-code.**

```
test("submits the form and shows confirmation", () => {
  fill(byLabel("Email"), "a@b.c")
  click(byRole("button", { name: "Submit" }))
  expect(byRole("status")).toHaveText("Submitted")
  expect(byRole("button", { name: "Submit" })).toBeDisabled()
})
```

**When to break it.** When two assertions describe two different behaviors, they really are two tests — split them.

## Pattern 8 — Test names read as specifications

**Principle.** A descriptive name is the bug report when the test fails. Vague names ("test1", "edge case") yield useless CI output.

**Evidence.**

- Go table-driven tests: names should *"describe input AND expected behavior"* — `"applies bulk discount when quantity exceeds 10"`, not `"100 dollars"`. (https://go.dev/wiki/TableDrivenTests)
- Pact: *"Describe the client's intent, not just the request type."* BDD-style given/when/then recommended. (https://docs.pact.io/consumer)

**Template.** `"should <outcome> when <condition> given <state>"`

## Pattern 9 — Table-driven / parameterized for input variation

**Principle.** Many tests differ only in inputs and expected outputs. Repeating the body invites drift; one canonical body iterated over data keeps logic in sync.

**Evidence.**

- Go: idiomatic `tests := []struct{ name, in, want }{...}; for _, tt := range tests { t.Run(tt.name, ...) }`. (https://go.dev/wiki/TableDrivenTests)
- Jest / Vitest: `test.each`. Pytest: `@pytest.mark.parametrize`. Playwright: array `.forEach`.

**Pseudo-code.**

```
cases = [
  { name: "positive",  in: (2, 3),   want: 5 },
  { name: "negatives", in: (-2, -3), want: -5 },
  { name: "zero",      in: (0, 0),   want: 0 },
]
for case in cases:
  test(case.name, () => expect(add(...case.in)).toEqual(case.want))
```

**When to break it.** When each case's setup or assertion logic genuinely differs, the table hides the difference. Then write separate named tests.

## Pattern 10 — Build test data via factories and builders

**Principle.** Real domain objects often have ~20 fields; tests care about 1–2. Literal duplication breeds breakage on schema changes and obscures which field actually matters.

**Evidence.** Pact: *"Use factories or fixtures to create the models for all your tests."* (https://docs.pact.io/consumer) Go: named-struct test cases + setup helpers with `t.Helper()` for cleanup pairs. (https://go.dev/wiki/TableDrivenTests)

**Pseudo-code (builder).**

```
user = aUser()
  .withEmail("a@b.c")
  .withRole("admin")
  .build()
```

**When to break it.** Trivially shaped data (a single int, a 2-field struct). A literal is clearer than indirection. Builders pay off at roughly five+ fields or when defaults need selective override.

## Pattern 11 — Mock at boundaries you do not control

**Principle.** Mocking your own services freezes implementation. Mocking third parties stops their outages from flaking the suite.

**Evidence.**

- Playwright: route-mock third-party endpoints; test against staging for the team's DB. (https://playwright.dev/docs/best-practices)
- Cypress: *"Only test websites that you control."* Use `cy.request()` against the team's server for setup; stub OAuth. (https://docs.cypress.io/app/core-concepts/best-practices)
- Pact: contract tests at the consumer / provider seam, not whole-stack. (https://docs.pact.io/consumer)
- Kent C. Dodds (Testing Trophy): *"Write tests. Not too many. Mostly integration."* (https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications)

**When to break it.** Never mock pure logic the team owns — call it directly.

## Pattern 12 — Page Object Model is a tool, not a religion

**Principle.** POMs encapsulate selectors and user flows for reuse, but bloat small suites, defeat recorders, and risk modeling DOM instead of user intent.

**Evidence.** Kyrre's pragmatic view: pick by app size; if used, keep it user-focused (`searchForProduct()`, not `clickSearchInput()`). Cons: boilerplate, learning curve, broken recorders. (https://www.kyrre.dev/blog/the-pragmatic-guide-to-playwright-testing)

**When to break it.** Small apps, prototypes, single-flow suites — inline locators are clearer.

## Test structure shapes

### Arrange–Act–Assert (AAA) / Given–When–Then (GWT)

Three-phase canonical shape. Cypress: *"You might also see this phrased as Given/When/Then, or Arrange/Act/Assert. The idea is the same."* (https://docs.cypress.io/app/end-to-end-testing/writing-your-first-end-to-end-test)

```
test("...", () => {
  // Arrange
  const user = aUser().build()
  render(<Profile user={user} />)
  // Act
  click(byRole("button", { name: "Edit" }))
  // Assert
  expect(byRole("textbox", { name: "Name" })).toBeFocused()
})
```

### Table-driven / parameterized

For input variation only. Each row is a named record; iterate as subtests so individual failures are addressable.

### BDD nesting

`describe(component) > describe(feature) > test(scenario)`. Pact's recommended template (https://docs.pact.io/consumer).

### Long E2E scenario

For real end-to-end flows (checkout, signup), one test with many assertions across steps is acceptable — splitting penalizes setup cost and proves nothing extra (https://docs.cypress.io/app/core-concepts/best-practices).

## Test data tradeoffs

| Pattern               | When to use                                       | Risk                                        |
| --------------------- | ------------------------------------------------- | ------------------------------------------- |
| Literal struct        | Simple, 1–2 fields, single use                    | Drift if schema changes                     |
| Named fixture (JSON)  | Larger payloads, shared across many tests         | Hides which fields matter; opaque diffs     |
| Factory function      | Default-valid object with overrides per test      | Indirection requires reading factory        |
| Builder               | Many optional fields, fluent override needed      | Boilerplate to maintain                     |
| Object Mother         | Named canonical scenarios (`anAdminUser`)         | Combinatorial blowup if not curated         |

Default to factory or builder for any domain entity; reserve literals for the field the test is actually about.

## Common framework gotchas

1. **Awaiting an assertion vs awaiting inside it.** Playwright: `await expect(loc).toBeVisible()` retries; `expect(await loc.isVisible()).toBe(true)` does not. (https://playwright.dev/docs/best-practices)
2. **`getBy` vs `findBy` vs `queryBy` (Testing Library).** `getBy` throws when missing, `queryBy` returns null (use for "assert not present"), `findBy` is async with built-in retry (use for "appears later"). (https://testing-library.com/docs/queries/about)
3. **Cypress commands are not synchronous.** `const x = cy.get(...)` does not work; use aliases. (https://docs.cypress.io/app/core-concepts/best-practices)
4. **Go parallel subtests need loop-variable capture.** `tt := tt` inside the loop body before `t.Parallel()`. Otherwise all subtests see the last iteration. (https://go.dev/wiki/TableDrivenTests)
5. **Snapshots are not for individual classes / attributes.** Use specific matchers (`toHaveClass`) instead; snapshots are for structural shape. (https://github.com/patternfly/patternfly-react/wiki/React-Testing-Library-Basics,-Best-Practices,-and-Guidelines)

## Sources

- Playwright — *Best Practices* — https://playwright.dev/docs/best-practices
- Testing Library — *About Queries* — https://testing-library.com/docs/queries/about
- Cypress — *Best Practices* — https://docs.cypress.io/app/core-concepts/best-practices
- Cypress — *Writing your first end-to-end test* — https://docs.cypress.io/app/end-to-end-testing/writing-your-first-end-to-end-test
- PatternFly — *React Testing Library Basics* — https://github.com/patternfly/patternfly-react/wiki/React-Testing-Library-Basics,-Best-Practices,-and-Guidelines
- Go Wiki — *Table-Driven Tests* — https://go.dev/wiki/TableDrivenTests
- OneUptime — *Go Table-Driven Tests* — https://oneuptime.com/blog/post/2026-01-07-go-table-driven-tests/view
- Kyrre — *Pragmatic Guide to Playwright Testing* — https://www.kyrre.dev/blog/the-pragmatic-guide-to-playwright-testing
- Pact — *Consumer guide* — https://docs.pact.io/consumer
- BugBug — *Cypress Best Practices* — https://bugbug.io/blog/testing-frameworks/cypress-best-practices
- Harness — *Flaky Tests: The Quiet Killer* — https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline
- Martin Fowler — *Practical Test Pyramid* — https://martinfowler.com/articles/practical-test-pyramid.html
- Kent C. Dodds — *The Testing Trophy* — https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications
- Trunk.io — *The Ultimate Guide to Flaky Tests* — https://trunk.io/blog/the-ultimate-guide-to-flaky-tests
