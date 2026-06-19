# Anti-Patterns — Twenty-Five Failure Modes Across Five Families

## Contents

- [How to use this catalog](#how-to-use-this-catalog)
- [Top seven](#top-seven)
- [Family A — Brittleness](#family-a--brittleness)
- [Family B — Flakiness](#family-b--flakiness)
- [Family C — Mock misuse](#family-c--mock-misuse)
- [Family D — Process pathologies](#family-d--process-pathologies)
- [Family E — AI-specific anti-patterns](#family-e--ai-specific-anti-patterns)
- [The framing question](#the-framing-question)
- [Sources](#sources)

## How to use this catalog

Each entry has six fields:

- **Violation** — the symptom.
- **Why wrong** — the consequence.
- **Fix** — the corrective pattern.
- **Gate question** — the one short question to ask *before* committing the code.
- **Evidence** — quoted source + URL.

The five families are descriptive, not exhaustive — several entries span two families. The top seven are bolded in the SKILL.md gist; they cause the most damage in modern codebases, especially when AI agents author the tests.

## Top seven

1. Mock-driven confidence (#25) — agent or human asserts on a value the same test wrote into a mock.
2. Brittle / implementation-detail selectors (#1) — tests bound to CSS classes, indices, xpath.
3. Static `sleep` / fixed-timeout waits (#7) — top empirical flakiness cause (~45%).
4. Test order dependency / hidden shared state (#8) — single most common root of correlated flake clusters.
5. Coverage as a vanity metric (#15) — drives almost every other process anti-pattern.
6. Happy-path-only coverage (#16) — particularly common in AI-generated tests.
7. Quarantine-as-cemetery + retry-as-fix (#21 + #22) — the team-level pathology that converts flakiness into silent coverage loss.

## Family A — Brittleness

Tests bound to internals. Survive only as long as nobody refactors.

### 1. Brittle / implementation-detail selectors

- **Violation.** `findElement(By.xpath("//div[2]/div[1]/button[@class='add-to-cart']"))` or `.btn.btn-large.primary`.
- **Why wrong.** Any layout or class-name refactor breaks the test even though user behavior is unchanged. *"Tests that break every time a class name shifts are not flaky; they are fragile by design"* (https://thinksys.com/qa-testing/how-to-reduce-flaky-tests-automation-frameworks).
- **Fix.** Selector hierarchy: role → label → text → test-id → structural. Stop at the highest rung that disambiguates.
- **Gate.** Would this test fail if the internals were refactored but user-visible behavior stayed identical?
- **Evidence.** *"Anti-Pattern: Using highly brittle selectors that are subject to change… Best Practice: Use `data-*` attributes."* (https://docs.cypress.io/app/core-concepts/best-practices)

### 2. Testing internal structure instead of observable behavior

- **Violation.** Asserting "method A was called, then B was called" rather than "given x, the output is z."
- **Why wrong.** Tests fail on every refactor; you lose the test's value as a regression net; encourages opposition to TDD.
- **Fix.** Restate the test as input → observable outcome.
- **Gate.** Am I asserting on what the code does, or on how it does it?
- **Evidence.** *"Don't reflect your internal code structure within your unit tests. Test for observable behaviour instead."* (https://martinfowler.com/articles/practical-test-pyramid.html)

### 3. Testing private methods directly

- **Violation.** Reflection or `package-private` exposure used to test a helper.
- **Why wrong.** Private methods exist only as a refactoring substrate; testing them ossifies internal structure.
- **Fix.** Drive the private method through the public method that uses it.
- **Gate.** Could this private method be renamed or deleted without changing public behavior? If yes, do not test it directly.
- **Evidence.** *"When you encounter a private method, locate the public method that calls the private method, and write your tests against the public method."* (https://learn.microsoft.com/en-us/dotnet/core/testing/unit-testing-best-practices)

### 4. Snapshot-as-test (snapshots replacing real assertions)

- **Violation.** Whole-component DOM snapshot is the only assertion; diffs are rubber-stamped.
- **Why wrong.** Encodes incidental markup as contract; locks in implementation; reviewers approve diffs they never read.
- **Fix.** Snapshots only for genuine structural concerns (element order, schema shape). Use explicit `toHaveClass`, `toBeDisabled` for specific properties.
- **Gate.** Could I describe in one sentence what this snapshot protects?
- **Evidence.** *"Snapshots should not be used to assert that a specific class is applied to a component. Tests should explicitly check for the class names… Snapshots should be used in situations where a component's structure is the primary concern."* (https://github.com/patternfly/patternfly-react/wiki/React-Testing-Library-Basics,-Best-Practices,-and-Guidelines)

### 5. Vague existence assertions

- **Violation.** `.should('exist')`, `expect(x).toBeTruthy()` with no content check.
- **Why wrong.** The test passes as long as anything renders; hides regressions in visibility, content, or state.
- **Fix.** Assert specific value, attribute, text, or state.
- **Gate.** If this assertion passes, what *exactly* did I prove?
- **Evidence.** *"Vague assertions like `.should('exist')` offer little value when debugging. They confirm presence but say nothing about content, attributes, or visibility."* (https://www.augustinfotech.com/blogs/cypress-assertions-best-practices-and-common-mistakes)

### 6. Action without assertion

- **Violation.** `cy.get('button').click();` end of test.
- **Why wrong.** Proves the framework can click; proves nothing about the system; passes when the click has zero effect.
- **Fix.** Always follow an action with an assertion about the resulting state.
- **Gate.** After this test passes, what about the product is now guaranteed?
- **Evidence.** *"Forgetting to assert after actions… Interacting with UI without validating the result makes your test pointless."* (https://www.augustinfotech.com/blogs/cypress-assertions-best-practices-and-common-mistakes)

## Family B — Flakiness

Tests that randomize their own verdicts. The corpus is unambiguous on the dominant causes.

### 7. Static `sleep` / fixed-timeout waits

- **Violation.** `cy.wait(2000)`, `Thread.sleep(5000)`, `page.waitForTimeout(2000)`.
- **Why wrong.** Slow on fast machines, still flaky on slow CI; papers over race conditions.
- **Fix.** Wait for the condition itself (`should`, `toPass`, `findBy`, `expect.poll`) and let the framework retry until satisfied.
- **Gate.** Am I waiting for an event, or for a guess at how long the event takes?
- **Evidence.** *"Replace `cy.wait()` with assertion retries… fixed waits result in slow, unreliable tests."* (https://www.augustinfotech.com/blogs/cypress-assertions-best-practices-and-common-mistakes) Empirical: *"developers thought they 'fixed' the flaky tests by increasing some time values… these time values actually have no effect."* (https://trunk.io/blog/the-ultimate-guide-to-flaky-tests)

### 8. Test order dependency / hidden shared state

- **Violation.** Test B implicitly relies on Test A's DB row, env var, or singleton.
- **Why wrong.** Becomes flaky under parallel runs, randomized order, or filtered runs.
- **Fix.** Each test sets up its own state via factories; per-test schemas or transaction rollback.
- **Gate.** Will this test pass when run alone with `.only`?
- **Evidence.** *"Anti-Pattern: Coupling multiple tests together… Tests should always be able to be run independently from one another and still pass."* (https://docs.cypress.io/app/core-concepts/best-practices) *"State leaking from one test to another is one of the biggest causes of flaky tests."* (https://trunk.io/blog/the-ultimate-guide-to-flaky-tests)

### 9. Non-deterministic inputs (clock, RNG, locale)

- **Violation.** `if (DateTime.Now.DayOfWeek == Tuesday)` in production with no seam; `Math.floor(Math.random()*10)` in tests; `new Date()` everywhere.
- **Why wrong.** Failures correlate with calendar, RNG seed, or timezone — invisible on dev machine, intermittent in CI.
- **Fix.** Inject clock and RNG seams; seed all RNGs; freeze time in tests.
- **Gate.** If this test runs Tuesday at 02:00 in Tokyo, does it still pass?
- **Evidence.** *"Mock the clock on the system. Instead of generated data, use specific test data with known properties."* (https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline)

## Family C — Mock misuse

Tests that test the test setup. The five entries from the original `test-antipatterns` skill all live here, expanded.

### 10. Asserting the mock exists (the original Anti-Pattern 1)

- **Violation.** `expect(screen.getByTestId('sidebar-mock')).toBeInTheDocument();`
- **Why wrong.** Verifies the mock works, not the component. Test passes when mock is present, fails when it is not. Tells you nothing about real behavior.
- **Fix.** Test the real component, or — when isolation is required — assert on the parent's behavior with the mocked child *present*, never on the mock itself.
- **Gate.** Am I testing real component behavior or just mock existence?
- **Evidence.** Original `test-antipatterns/SKILL.md` (this repository, pre-rewrite). Reinforced by *"Mocks are tools to isolate, not things to test."* (https://martinfowler.com/articles/practical-test-pyramid.html)

### 11. Mock drift (mock no longer matches real API)

- **Violation.** Mock returns a hand-coded shape that no longer matches the real service.
- **Why wrong.** Tests pass against a fiction; production fails on first real call.
- **Fix.** Contract tests (Pact) or generated fixtures from real responses; pair every mocked boundary with at least one integration test.
- **Gate.** When the real API changed last quarter, would this test have caught it?
- **Evidence.** *"Tests can fail silently if mocked behavior drifts too far from how real services behave."* (https://qase.io/blog/a-comprehensive-guide-to-api-testing-best-practices-tools-challenges-and-more) *"Mocking is a decent strategy for testing specific behaviors, but it should not be used as a replacement for end-to-end testing."* (https://trunk.io/blog/the-ultimate-guide-to-flaky-tests)

### 12. Over-mocking child components

- **Violation.** Every child, hook, and provider mocked; the test renders only the parent's own JSX.
- **Why wrong.** Stops testing the integration users depend on; integration bugs hide between mocks.
- **Fix.** Default to real children; only mock for genuine isolation needs (slow I/O, randomness).
- **Gate.** What integration does this test still cover after I add this mock?
- **Evidence.** *"Avoid Over-Mocking Child Components: Mocking too much can hide integration issues; prefer real child components."* (https://birdeatsbug.com/blog/cypress-component-testing)

### 13. Incomplete mocks (original Anti-Pattern 4)

- **Violation.** Partial mock with only the fields the test author thought to include; downstream code consumes missing fields and breaks silently.
- **Why wrong.** Partial mocks hide structural assumptions. Tests pass; integration fails.
- **Fix.** Mirror the real API completeness — every field the system consumes downstream.
- **Gate.** What fields does the real API response contain, and is every consumed field present?
- **Evidence.** Original `test-antipatterns/SKILL.md`; reinforced by mock-drift evidence above.

### 14. Mocking without understanding (original Anti-Pattern 3)

- **Violation.** Mock at a high level that the test secretly depends on for a side effect (e.g., writing a config file).
- **Why wrong.** Mocked method had a side effect the test depended on; test passes for the wrong reason or fails mysteriously.
- **Fix.** Run the test with the real implementation first, observe what side effects matter, then mock at the *lower* level (the slow / external operation) instead of the high-level method.
- **Gate.** What side effects does the real method have, and does this test depend on any of them?
- **Evidence.** Original `test-antipatterns/SKILL.md`; reinforced by Microsoft mocking guidance (https://learn.microsoft.com/en-us/dotnet/core/testing/unit-testing-best-practices).

## Family D — Process pathologies

The team-level and suite-level failure modes that compound over months.

### 15. Coverage as a vanity metric

- **Violation.** Line% is the success criterion; trivial tests fill the gap.
- **Why wrong.** Coverage measures executed code, not protected behaviors; rewards assertion-free or assertion-weak tests.
- **Fix.** Use mutation testing on critical modules. Coverage is a floor, never a goal. Skip trivial code entirely.
- **Gate.** If I delete the body of this function, do the tests still pass?
- **Evidence.** *"A high code coverage percentage isn't an indicator of success."* (https://learn.microsoft.com/en-us/dotnet/core/testing/unit-testing-best-practices) *"Code coverage tells you what code was executed, but not how well your tests can actually detect problems."* (https://testrigor.com/blog/understanding-mutation-testing-a-comprehensive-guide)

### 16. Happy-path-only coverage

- **Violation.** All cases use well-formed inputs; no invalid input, error path, empty state, or boundary tested.
- **Why wrong.** Real bugs hide in malformed input and edges; scores look high but reliability does not.
- **Fix.** Pair every happy-path test with one negative or boundary test. AI-generated suites are especially prone to this.
- **Gate.** Have I tested empty, null, malformed, oversized input?
- **Evidence.** *"Evaluating only happy paths: If your task suite mostly contains well-formed inputs that the agent handles easily, you'll get high scores that don't reflect real-world performance. Deliberate adversarial testing is not optional."* (https://www.mindstudio.ai/blog/ai-agent-custom-benchmarks-evaluation)

### 17. Eternal `beforeAll` / shared setup across tests

- **Violation.** Heavy setup at the suite level; tests pile up implicit dependencies on the shared object.
- **Why wrong.** Hard to read; tests influence each other; over-setup obscures intent.
- **Fix.** Prefer helper methods or factories called per-test over `Setup`/`TearDown` attributes.
- **Gate.** Could a new reader understand each test without scrolling to the top of the file?
- **Evidence.** *"Use helper methods instead of `Setup` and `Teardown`… less chance of sharing state between tests."* (https://learn.microsoft.com/en-us/dotnet/core/testing/unit-testing-best-practices)

### 18. Cleanup in `afterEach`

- **Violation.** Database reset, logout, file cleanup placed in `afterEach`.
- **Why wrong.** If the test crashes or is interrupted, cleanup never runs and the next test starts dirty.
- **Fix.** Move "must run" cleanup into `beforeEach` (re-seeding).
- **Gate.** If the test crashes halfway, does the next test still start clean?
- **Evidence.** *"Anti-Pattern: Using `after` or `afterEach` hooks to clean up state… there is no guarantee that this code will run."* (https://docs.cypress.io/app/core-concepts/best-practices)

### 19. Magic strings and logic in tests

- **Violation.** Test asserts `"1001"` with no comment; or builds expected values via a loop with branching.
- **Why wrong.** Readers cannot tell why the value matters; logic in tests can have bugs.
- **Fix.** Promote magic values to named constants. Use parameterized tests instead of in-test loops.
- **Gate.** If a teammate read only this test, would they know what behavior it locks in?
- **Evidence.** *"Magic strings… make your code less readable and harder to maintain… Avoid coding logic in unit tests… the chance of introducing bugs increases dramatically."* (https://learn.microsoft.com/en-us/dotnet/core/testing/unit-testing-best-practices)

### 20. Testing against third-party sites you do not control

- **Violation.** E2E logs in via Google OAuth UI or scrapes a third-party page.
- **Why wrong.** Slow; A/B variants break tests; rate-limited; bot-detected; flaky for reasons unrelated to the team's code.
- **Fix.** Stub at the network layer; only test domains the team owns; use APIs, not UI, for third-party interactions.
- **Gate.** Whose servers does this test depend on, and what happens when they change?
- **Evidence.** *"Anti-Pattern: Trying to visit or interact with sites or servers you do not control."* (https://docs.cypress.io/app/core-concepts/best-practices)

### 21. Quarantine-as-cemetery

- **Violation.** Failing tests marked `skip` / `@Ignore` / quarantined indefinitely; no owner, no fix-by date.
- **Why wrong.** Coverage looks the same, but is silently gone. The suite stops catching the regressions it was meant for.
- **Fix.** Every quarantined test gets a named owner, ticket, SLA, and re-entry gate (e.g., ten consecutive clean runs).
- **Gate.** Who owns this skipped test, and what condition restores it?
- **Evidence.** *"Quarantined tests stop helping with regression coverage, and without accountability, the quarantine folder becomes a one-way door."* (https://thinksys.com/qa-testing/how-to-reduce-flaky-tests-automation-frameworks)

### 22. Retry-as-fix (auto-retry hiding real bugs)

- **Violation.** Failed tests auto-retried 3–10 times; team treats retried-then-passed as green.
- **Why wrong.** A correct test against flaky product code looks identical to a flaky test; real bugs ship.
- **Fix.** Retry only with telemetry. Record every retry. Fix root cause behind clusters. Quarantine instead of silently retrying.
- **Gate.** Is this retry buying reliability, or hiding a real failure?
- **Evidence.** *"A perfectly correct, stable test covering flaky product code looks exactly like a flaky test and so real issues can easily be hidden by retries."* (https://gradle.com/blog/a-pragmatists-guide-to-flaky-test-management)

### 23. Duplicate tests across pyramid layers

- **Violation.** Same logic asserted at unit, integration, and E2E.
- **Why wrong.** Adds maintenance burden, slows the suite, multiplies failures from a single change without adding confidence.
- **Fix.** Push tests down the pyramid; higher layers only cover what lower layers cannot.
- **Gate.** Does this higher-level test add confidence the lower test could not?
- **Evidence.** *"Avoid Test Duplication… duplicating tests throughout the different layers of the pyramid… every single test is additional baggage."* (https://martinfowler.com/articles/practical-test-pyramid.html)

### 24. Weakening tests to make them pass

- **Violation.** Loosening assertions, adding tolerances, branching `if` blocks so a red test goes green without touching production code.
- **Why wrong.** Tests exist to catch bugs; weakening them sacrifices correctness; the bug ships.
- **Fix.** A failing test is a signal — fix the production code or, if behavior intentionally changed, redesign the test. Never erode assertion strength.
- **Gate.** Am I changing the test because the contract changed, or because the test caught me?
- **Evidence.** *"Test code is as important as production code. Give it the same level of care and attention. 'This is only test code' is not a valid excuse."* (https://martinfowler.com/articles/practical-test-pyramid.html)

### 25. Test-only methods in production (original Anti-Pattern 2)

- **Violation.** Production class grows a `destroy()` / `reset()` / `_forTest` method used only by the test suite.
- **Why wrong.** Pollutes production API; dangerous if called for real; violates separation of concerns; confuses object lifecycle with entity lifecycle.
- **Fix.** Move cleanup helpers into test utilities; redesign for injectable resource ownership.
- **Gate.** Is this method called from any non-test path? Does this class own this resource's lifecycle?
- **Evidence.** Original `test-antipatterns/SKILL.md`.

### Integration tests as afterthought (original Anti-Pattern 5)

This one spans the whole catalog rather than naming a specific entry. The original framing:

> Tests are part of implementation, not optional follow-up. Cannot claim complete without tests. Tests written alongside implementation catch issues early.

Operational rule: a feature is not "done" until at least one integration-layer test exercises it against real systems — the same gate humans owe applies to coding agents.

## Family E — AI-specific anti-patterns

These overlap with the families above but warrant their own naming because coding agents produce them at scale.

### Mock-driven confidence

- **Violation.** Agent generates a test, mocks every dependency, asserts the mock's return — the test cannot fail unless the agent's mock setup contradicts itself. The test pins what the agent *thinks* the code does, not what production does.
- **Why wrong.** *"Metric green, user red."* The agent is grading itself.
- **Fix.** Apply the Mock Budget Rule: unit tests at I/O boundaries may mock the boundary only. A test that mocks every collaborator named in the SUT must also have one integration-layer companion test that exercises the same path without mocks. Forbid asserting on a value the same test body wrote into a mock.
- **Gate.** If I deleted the real code under test and kept the mock, would this test still pass?
- **Evidence.** Original `test-antipatterns/SKILL.md` framing; Yoshimoto et al., *Testing with AI Agents: An Empirical Study* (https://arxiv.org/html/2603.13724). *"AI-generated tests feature longer code and higher assertion density than human-written tests, while maintaining lower cyclomatic complexity."* — same paper.

### Assertion roulette in agent-generated tests

- **Violation.** A single test asserts many unrelated facts; when it fails, you cannot tell which fact broke. AI-generated tests show this strongly — median assertions per test is 2.0 for AI vs 1.0 for humans (Yoshimoto et al.).
- **Why wrong.** The first failed assertion stops execution, masking the others; diagnosis becomes guesswork.
- **Fix.** One logical assertion per test (Pattern 7); use table-driven tests for many inputs (Pattern 9).
- **Gate.** Will the failure message tell me exactly which behavior broke?
- **Evidence.** *"AI agents may be prone to the Assertion Roulette test smell — a known bad practice where multiple assertions in a single test make it difficult to pinpoint the exact cause of failure."* (https://arxiv.org/html/2603.13724)

### Verbose-but-linear AI tests

- **Violation.** Agent emits long tests with many assertions and very low cyclomatic complexity but no branch or edge coverage.
- **Why wrong.** Verification density looks high; fault-detection capability is unverified; assertion-roulette debt accumulates.
- **Fix.** Treat AI tests as junior PRs — review for assertion roulette, missing edge cases, and mutation-test the output.
- **Gate.** Did the agent test the *failure modes* of this code, or just walk the happy path twice?
- **Evidence.** *"AI-generated tests exhibit longer code and higher density of assertions while maintaining lower cyclomatic complexity through linear logic."* (https://arxiv.org/html/2603.13724)

### Coverage gaming by agents

- **Violation.** Agent writes tests that call lines without checking behavior.
- **Why wrong.** Coverage gain is not a behavior gain. Coverage is a presence proxy, not a correctness oracle.
- **Fix.** Negative companion rule — every agent-generated test ships with a negative counterpart that asserts the SUT *rejects* invalid input. If deleting the negative assertion still leaves the suite green, the negative was hollow.
- **Gate.** If I delete the negative assertion, does the suite still pass for the positive?
- **Evidence.** Yoshimoto et al. report AI-generated tests improve coverage in 75% of commits in some projects — without proportional behavior gains.

### Snapshot / golden-file abuse by agents

- **Violation.** When agents generate snapshots, they freeze whatever the code happens to output now. The snapshot becomes a tautology.
- **Why wrong.** Locks in implementation; reviewers do not read snapshot diffs.
- **Fix.** Snapshot only for `PRODUCT_CONTRACT` artifacts the product itself ships (public API schema, generated SDK file). Forbid snapshots of formatted UI text, error messages, CSS literal values, internal JSON shapes. If uncertain, classify as `IMPLEMENTATION_DETAIL` by default and use specific matchers.
- **Gate.** Is this artifact a `PRODUCT_CONTRACT` or an `IMPLEMENTATION_DETAIL`?
- **Evidence.** Original `test-antipatterns` framing + Anthropic eval guide caution about over-rigid grading (https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents).

## The framing question

From the original `test-antipatterns` skill, preserved verbatim because it still works as a fast smell-check during review:

> *"Are we testing the behavior of a mock?"*
>
> *"Do we need to be using a mock here?"*

If either question makes the test author hesitate, the test almost always belongs elsewhere.

## Sources

- Yoshimoto et al. — *Testing with AI Agents: An Empirical Study* — https://arxiv.org/html/2603.13724
- Martin Fowler — *The Practical Test Pyramid* — https://martinfowler.com/articles/practical-test-pyramid.html
- Kent C. Dodds — *The Testing Trophy* — https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications
- Microsoft — *Unit testing best practices for .NET* — https://learn.microsoft.com/en-us/dotnet/core/testing/unit-testing-best-practices
- Cypress — *Best Practices* — https://docs.cypress.io/app/core-concepts/best-practices
- August Infotech — *Cypress Assertions: Best Practices and Common Mistakes* — https://www.augustinfotech.com/blogs/cypress-assertions-best-practices-and-common-mistakes
- BirdEats — *Cypress Component Testing* — https://birdeatsbug.com/blog/cypress-component-testing
- Thinksys — *How to reduce flaky tests* — https://thinksys.com/qa-testing/how-to-reduce-flaky-tests-automation-frameworks
- Trunk.io — *The Ultimate Guide to Flaky Tests* — https://trunk.io/blog/the-ultimate-guide-to-flaky-tests
- Harness — *Flaky Tests: The Quiet Killer* — https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline
- Talent500 — *Fixing Flaky Tests in Test Automation* — https://talent500.com/blog/fixing-flaky-tests-in-test-automation
- Qase — *Comprehensive Guide to API Testing* — https://qase.io/blog/a-comprehensive-guide-to-api-testing-best-practices-tools-challenges-and-more
- Mindstudio — *AI Agent Custom Benchmarks Evaluation* — https://www.mindstudio.ai/blog/ai-agent-custom-benchmarks-evaluation
- testRigor — *Understanding Mutation Testing* — https://testrigor.com/blog/understanding-mutation-testing-a-comprehensive-guide
- PatternFly — *React Testing Library Basics* — https://github.com/patternfly/patternfly-react/wiki/React-Testing-Library-Basics,-Best-Practices,-and-Guidelines
- Kyrre — *Pragmatic Guide to Playwright Testing* — https://www.kyrre.dev/blog/the-pragmatic-guide-to-playwright-testing
- Anthropic — *Demystifying Evals for AI Agents* — https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents
- Gradle — *A Pragmatist's Guide to Flaky Test Management* — https://gradle.com/blog/a-pragmatists-guide-to-flaky-test-management
