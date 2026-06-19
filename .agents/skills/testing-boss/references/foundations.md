# Foundations — Decide Where, Why, and Whether to Test

## Contents

- [Premise](#premise)
- [Principle 1 — Name the invariant before opening a test file](#principle-1--name-the-invariant-before-opening-a-test-file)
- [Principle 2 — Push every test to the lowest layer that can detect the failure](#principle-2--push-every-test-to-the-lowest-layer-that-can-detect-the-failure)
- [Principle 3 — Risk × likelihood drives effort](#principle-3--risk--likelihood-drives-effort)
- [Principle 4 — Test observable behavior, not implementation](#principle-4--test-observable-behavior-not-implementation)
- [Principle 5 — Coverage is a flashlight, not a target](#principle-5--coverage-is-a-flashlight-not-a-target)
- [Principle 6 — Declare the test boundary explicitly](#principle-6--declare-the-test-boundary-explicitly)
- [Principle 7 — Micro and macro, never only one](#principle-7--micro-and-macro-never-only-one)
- [The pyramid vs trophy debate is mostly semantic noise](#the-pyramid-vs-trophy-debate-is-mostly-semantic-noise)
- [Placement decision tree](#placement-decision-tree)
- [Test header template](#test-header-template)
- [Delete tests aggressively](#delete-tests-aggressively)
- [Sources](#sources)

## Premise

Most bad tests are placement failures — wrong layer, wrong invariant, wrong boundary — not assertion failures. Before any line of test code, the author answers three questions:

1. What invariant am I protecting?
2. What is the lowest layer that can fail when this invariant breaks?
3. Is the risk-weighted cost of this bug high enough to justify the test plus its lifetime maintenance?

If any answer is fuzzy, do not write the test. Sharpen the answer first.

## Principle 1 — Name the invariant before opening a test file

Force a one-sentence invariant before any test code. Example invariants:

- "`invoice.total` never exceeds the sum of line-item subtotals after discounts and taxes."
- "A successful `POST /refund` reduces the customer's stored balance by exactly the refund amount."
- "The login screen rejects any input that does not contain `@` with the message `Enter a valid email`."

A fuzzy invariant ("the form works", "the discount applies correctly") guarantees a fuzzy test. The skill rejects test creation until the invariant fits one sentence with concrete nouns.

The user's project doctrine (CLAUDE.md, MOST_CRITICAL section) requires this explicitly: *"Before adding, moving, or expanding any test, identify the invariant, the owning layer, and the existing canonical suite for that layer."*

## Principle 2 — Push every test to the lowest layer that can detect the failure

> "If a higher-level test spots an error and there's no lower-level test failing, you need to write a lower-level test."
> — Martin Fowler, *The Practical Test Pyramid* (https://martinfowler.com/articles/practical-test-pyramid.html)

> "Push your tests as far down the test pyramid as you can."
> — same source

> "I delete high-level tests that are already covered on a lower level… Beware of the sunk cost fallacy and hit the delete key."
> — same source

Operational rule: when a higher-layer test fires *and* no lower-layer test catches the same defect, that is a placement bug. Write the lower-layer test, verify it fails for the same reason, and delete the higher-layer test unless it covers additional behavior the lower layer cannot reach (e.g., wiring across processes).

## Principle 3 — Risk × likelihood drives effort

> "Risk-based testing helps teams work smarter by testing what's most likely to break and cause serious problems."
> — Tricentis, *A detailed guide to risk-based testing* (https://www.tricentis.com/learn/risk-based-testing)

> "Not all features in a software system are equally important or risky… Prioritization ensures you're testing what's most likely to fail and what would cause the most damage if anything fails."
> — same source

Score every candidate test on two axes: probability of bug × blast radius if shipped. Below a threshold, skip or absorb into a broader smoke test. Above it, the test must live at the *narrowest* owning layer. Some code legitimately has no tests — the highest-leverage placement decision is sometimes "do not write this test."

## Principle 4 — Test observable behavior, not implementation

> "Don't reflect your internal code structure within your unit tests. Test for observable behaviour instead."
> — Fowler, *The Practical Test Pyramid* (https://martinfowler.com/articles/practical-test-pyramid.html)

> "Private methods should generally be considered an implementation detail. That's why you shouldn't even have the urge to test them."
> — same source

> "Implementation-focused: `expect(page.locator('#error-message.visible')).toBeVisible();` ✅ User-focused: `expect(page.getByText('Please enter a valid email')).toBeVisible();`"
> — Kyrre, *Pragmatic Guide to Playwright Testing* (https://www.kyrre.dev/blog/the-pragmatic-guide-to-playwright-testing)

Reject any test whose name reads "calls method X" or "passes Y to internal helper". Replace with "given input → produces outcome / observable state change".

## Principle 5 — Coverage is a flashlight, not a target

> "A high code coverage percentage isn't an indicator of success, and it doesn't imply high code quality."
> — Microsoft, *Unit testing best practices for .NET* (https://learn.microsoft.com/en-us/dotnet/core/testing/unit-testing-best-practices)

> "Code coverage tells you what code was executed, but not how well your tests can actually detect problems."
> — testRigor, *Understanding Mutation Testing* (https://testrigor.com/blog/understanding-mutation-testing-a-comprehensive-guide)

> "No test is inherently useful just because it exists."
> — Artem Zakharchenko, *The True Purpose of Testing* (https://www.epicweb.dev/the-true-purpose-of-testing)

Use coverage to *find dark code*, not to score quality. For each uncovered region, decide "test or delete" — do not pad with assertion-free tests. Use mutation testing on the critical modules to measure suite *strength* rather than presence:

```
mutation_score = killed_mutants / (killed + surviving - equivalent)
gate := mutation_score(critical_modules) >= 80
```

## Principle 6 — Declare the test boundary explicitly

> "Automated tests rarely involve your entire system… There's often a place where you draw the line. The boundary."
> — Artem Zakharchenko, *What is a test boundary* (https://www.epicweb.dev/what-is-a-test-boundary)

Each test file (or `describe` block) names what is inside the boundary and what is outside. Implicit boundaries breed duplicate or contradictory tests across layers. The template:

```
// Suite boundary
// IN:  the discount calculator pure function
// OUT: HTTP, database, downstream pricing service (owned by /tests/integration/pricing.test)
// Invariant: discount never exceeds the configured cap
```

This single comment block resolves about 80% of "why does this test cover that?" arguments.

## Principle 7 — Micro and macro, never only one

> "Most teams focus only on macro-level testing, which usually results in multiple issues in production… when item prices in production are negative or show an unexpected number of decimals, order creation breaks."
> — Thoughtworks, *Seven guiding principles in testing* (https://www.thoughtworks.com/en-us/insights/blog/testing/seven-guiding-principles-testing)

Every feature ships with at least one micro test (a property / boundary / edge case) **and** one macro test (a real user flow). If only one feels affordable, the invariants are not yet sharp enough.

## The pyramid vs trophy debate is mostly semantic noise

> "People love debating what percentage of which type of tests to write, but it's a distraction. Nearly zero teams write expressive tests that establish clear boundaries, run quickly & reliably, and only fail for useful reasons. Focus on that instead."
> — Martin Fowler, *On the Diverse And Fantastical Shapes of Testing* (https://martinfowler.com/articles/2021-test-shapes.html)

> "Honeycomb advocates' 'unit test' is what I'd call 'solitary' and their 'integration test' is what I'd call 'sociable' — so the pyramid-vs-honeycomb argument is largely moot."
> — same source

> "The more your tests resemble the way your software is used, the more confidence they can give you."
> — Kent C. Dodds, *The Testing Trophy and Testing Classifications* (https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications)

The trophy is the pyramid with the word "unit" redefined to mean *sociable* (collaborators left in). Both shapes are healthy when:

- Lower layers are fast and isolated.
- Higher layers prove integrations the lower layers cannot.
- No invariant is duplicated across layers without an explicit reason.

Argue about shape *only* after the boundary contract for each suite is named. Otherwise the debate hides the real placement bugs.

Important caveat from Dodds himself: he restricts the Trophy framing to *single-codebase or monolith* views. Do not invoke "trophy" to justify cross-microservice integration testing.

## Placement decision tree

```
1. What is the invariant? (one sentence, concrete nouns)
2. Can a pure-function unit test fail when the invariant breaks?
     YES → unit test owns it. STOP.
     NO  → go to 3.
3. Can a service / module test (real internals, mocked external boundaries)
   fail when the invariant breaks?
     YES → integration (sociable) test owns it. STOP.
     NO  → go to 4.
4. Does the invariant require crossing a process / network / browser boundary?
     YES → route-level or E2E test owns it. STOP.
     NO  → the invariant is mis-stated; return to step 1.
```

Always pick the earliest STOP. The higher you go, the slower the feedback and the wider the failure surface.

## Test header template

Every test file leads with a four-line header. It costs ten seconds and prevents most boundary disputes:

```
// Suite: <one short name>
// Invariant: <one sentence>
// Boundary IN: <what this suite is responsible for>
// Boundary OUT: <what is owned by another suite, with path>
```

## Delete tests aggressively

The sunk-cost fallacy is real in test suites. A redundant or implementation-coupled test is a maintenance tax with no benefit. When:

- The same invariant is asserted at a lower layer, OR
- The test fails on every refactor unrelated to its invariant, OR
- The test is permanently skipped or quarantined without an owner,

delete it. Fowler is explicit: *"Beware of the sunk cost fallacy and hit the delete key."* (https://martinfowler.com/articles/practical-test-pyramid.html)

## Sources

- Martin Fowler — *The Practical Test Pyramid* — https://martinfowler.com/articles/practical-test-pyramid.html
- Martin Fowler — *On the Diverse And Fantastical Shapes of Testing* — https://martinfowler.com/articles/2021-test-shapes.html
- Kent C. Dodds — *The Testing Trophy and Testing Classifications* — https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications
- Microsoft — *Unit testing best practices for .NET* — https://learn.microsoft.com/en-us/dotnet/core/testing/unit-testing-best-practices
- Tricentis — *A detailed guide to risk-based testing* — https://www.tricentis.com/learn/risk-based-testing
- Thoughtworks — *Seven guiding principles in testing* — https://www.thoughtworks.com/en-us/insights/blog/testing/seven-guiding-principles-testing
- Artem Zakharchenko — *The True Purpose of Testing* — https://www.epicweb.dev/the-true-purpose-of-testing
- Artem Zakharchenko — *What is a test boundary* — https://www.epicweb.dev/what-is-a-test-boundary
- testRigor — *Understanding Mutation Testing* — https://testrigor.com/blog/understanding-mutation-testing-a-comprehensive-guide
- Kyrre — *Pragmatic Guide to Playwright Testing* — https://www.kyrre.dev/blog/the-pragmatic-guide-to-playwright-testing
- LexingtonSoft — *Code Coverage Best Practices* (PDF) — https://www.verifysoft.com/Code-Coverage-Best-Practices-LexingtonSoft-white-paper.pdf
- QASphere — *Exploratory Testing: A Practical Guide* — https://qasphere.com/blog/exploratory-testing-guide
