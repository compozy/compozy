# CI and Automation — Flaky Discipline, Gates, and Test Quality Probes

## Contents

- [Flaky-test taxonomy](#flaky-test-taxonomy)
- [Quarantine-first flaky workflow](#quarantine-first-flaky-workflow)
- [Flaky-rate as a first-class CI SLO](#flaky-rate-as-a-first-class-ci-slo)
- [CI stage pyramid](#ci-stage-pyramid)
- [Deterministic test architecture](#deterministic-test-architecture)
- [Per-test isolated state](#per-test-isolated-state)
- [Contract testing instead of broad E2E](#contract-testing-instead-of-broad-e2e)
- [Narrow integration tests plus contract tests](#narrow-integration-tests-plus-contract-tests)
- [Mutation testing as a suite-quality probe](#mutation-testing-as-a-suite-quality-probe)
- [Risk-based regression packs](#risk-based-regression-packs)
- [Two-tier accessibility coverage](#two-tier-accessibility-coverage)
- [Real-system gates vs mocks (reconciled)](#real-system-gates-vs-mocks-reconciled)
- [Hot takes worth adopting](#hot-takes-worth-adopting)
- [Sources](#sources)

## Flaky-test taxonomy

Empirical cause distribution from large CI corpora:

| Cause family            | Share | Detect                                                                 | Fix (not retry)                                                                            |
| ----------------------- | ----: | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| Async / timing          |  ~45% | Run test N× under runner load; cluster failures by time-of-day         | Replace `sleep` with `wait_until(predicate, timeout)`; await response, not wall-clock      |
| Concurrency / race      |  ~20% | Stress test with `-race` / TSan; reproduce 100×                        | Add locks; make ops deterministic; serialize the critical region                           |
| Test order / pollution  |  ~12% | Shuffle test order; run subsets in isolation                           | Per-test schemas / temp-dirs; explicit teardown; no module-level singletons                |
| External resource       |       | Test fails only when network or runner is loaded                       | Mock the boundary; add resource health check before test body                              |
| Non-determinism (RNG)   |       | Failures cluster on certain inputs                                     | Seed RNG, inject FakeClock, use fixed test data                                            |
| Infrastructure          |       | Failures correlate with one runner pool                                | Standardize runner image; pin memory / CPU; treat infra as code                            |

Cited from Trunk.io's *Ultimate Guide to Flaky Tests* (https://trunk.io/blog/the-ultimate-guide-to-flaky-tests) and Harness *Flaky Tests: The Quiet Killer* (https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline). Key quote: *"Developers thought they 'fixed' the flaky tests by increasing some time values… these time values actually have no effect."* (Wing Lam et al., via Trunk.io.)

## Quarantine-first flaky workflow

Re-runs and disabling tests erode signal quality and trust. The pattern that works:

1. **Detect** automatically when a test's pass/fail toggles on the same SHA.
2. **Quarantine** — move the test to a bucket that keeps running but does not block merge.
3. **Assign a named owner within 24 hours**. Not a team — a person.
4. **Set a fix-by date**. Quarantine without a deadline becomes a one-way door.
5. **Re-entry gate** — for example, ten consecutive clean runs before the test rejoins the main suite.

**Pseudo-config:**

```
on test_outcome_history(test_id):
  if flakiness_rate(last_50_runs) > 1%:
    tag(quarantine)
    open_ticket(owner = test_owner)

pipeline.merge_gate := failures_in(main_suite)
                       AND NOT failures_in(quarantine)
```

**Evidence.** *"The answer is automatic quarantine. Put a flaky test in quarantine so it can still run, but doesn't block the pipeline."* (https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline) *"Every flaky test gets a named owner within 24 hours of identification: Not a team. A person."* (https://thinksys.com/qa-testing/how-to-reduce-flaky-tests-automation-frameworks) Slack's Project Cornflake reduced test-job failure from 56.76% to 3.85% in less than a year using this loop (https://trunk.io/blog/the-ultimate-guide-to-flaky-tests).

## Flaky-rate as a first-class CI SLO

Treat `flaky_rate` as an operational metric next to build duration.

- **SLO**: < 1–2%
- **Alert**: > 5%
- **Trigger** for retro / dedicated cleanup: > 3% for two consecutive weeks

Evidence: *"Treat flaky test rate as a top operational metric. Healthy test suites keep flaky rates below 1–2%, while rates above 5% show that there are big problems."* (https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline)

## CI stage pyramid

The gate sequence (run order = fast-to-slow + cheap-to-expensive):

1. **Pre-commit (local)** — lint, format, type-check, fastest unit subset.
2. **Build stage (every PR)** — full unit suite (~70% of all tests), static analysis, secrets scan, SCA, SBOM, contract tests as consumer. Fast-fail.
3. **Staging stage (post-merge or pre-deploy)** — narrow integration tests, provider-side contract verification, broader integration, performance smoke, DAST. Real DB / queue containers (testcontainers-style) over in-memory fakes.
4. **Pre-prod gate** — end-to-end smoke covering top user journeys; accessibility scan; visual diff if maintained.
5. **Production** — canary + synthetic monitoring + QA-in-prod signals.

Evidence: *"Testing should start as early as possible… Unit tests should make up the bulk of your testing strategy. A good rule of thumb is about 70 percent."* (https://docs.aws.amazon.com/whitepapers/latest/practicing-continuous-integration-continuous-delivery/testing-stages-in-continuous-integration-and-continuous-delivery.html)

**Anti-patterns in CI gate design** (each one observed in the corpus):

- Monolithic "test" job that mixes fast unit and slow integration. *"When you mix fast, predictable unit tests with slow, environment-dependent integration tests, the integration test flakiness spreads to everything else."* (https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline)
- Gates that pass on docs-only diffs without scoping.
- Retry-until-green loops at the pipeline level.
- E2E suites as the primary gate. *"E2E tests can be slow… fragile… coordinating large all-in-one deployments."* (https://pactflow.io/blog/contract-testing-vs-integration-testing)
- Quarantine bucket with no deadlines.

## Deterministic test architecture

Async waits, real wall-clock, real RNG, shared singletons cause race-condition flakiness. Counter with seams.

```
clock := FakeClock(t0)
rng   := SeededRNG(42)
wait_until(predicate, timeout = 5s)
```

- Dependency-inject `clock`, `rng`, and IO.
- Seed every generator.
- Replace `sleep` with explicit "wait until condition" with timeout.

Evidence: *"Async Wait (45%), Concurrency (20%), and Test Order Dependency (12%)… Async Wait: 54% are fixed by awaiting a response instead of static timeouts."* (https://trunk.io/blog/the-ultimate-guide-to-flaky-tests) *"Seed random number generators. Mock the clock on the system."* (https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline)

## Per-test isolated state

Parallel tests collide on shared DB rows, files, env vars. Each test owns a unique temp directory, schema, or namespace; teardown wipes it.

```
setup:
  schema = "test_" + uuid()
  migrate(schema)
  db.set_search_path(schema)

teardown:
  drop_schema(schema)
```

Evidence: *"Create temporary directories, test-specific namespaces, and database schemas. After tests, clean up the resources. Don't use global shared state."* (https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline) *"Robust tests are completely isolated from other tests. They each set up their own state."* (https://trunk.io/blog/the-ultimate-guide-to-flaky-tests)

## Contract testing instead of broad E2E

E2E suites between services scale exponentially with integrations and need shared envs. Contract tests scale linearly.

**Consumer-driven contract pattern (Pact-style):**

```
// consumer test
mock_provider
  .given("user exists")
  .upon("GET /users/1")
  .will_respond({ id: 1, name: "x" })

publish_pact("consumer-X", "provider-Y")

// provider build
for pact in pacts_for("provider-Y"):
  replay(pact).assert_matches(actual_response)
```

**Cost.** Contracts do not cover side effects or full workflows; only the integration shape.

Evidence: *"Tests scale linearly with the number of integrations, instead of exponentially."* (https://pactflow.io/blog/contract-testing-vs-integration-testing) *"Use of an explicit contract is best when the provider is able to incorporate the contract verification step into its own build."* (https://pactflow.io/what-is-consumer-driven-contract-testing)

## Narrow integration tests plus contract tests

"Integration test" is overloaded; broad E2E is slow and fragile. The combination that works:

- **Narrow integration tests** exercise the boundary code against a test double (in-process fake or stub).
- **Contract tests** guarantee that double matches the real service.

Evidence: *"Using this combination of using narrow integration tests and contract tests, I can be confident of integrating against an external service without ever running tests against a real instance."* (https://martinfowler.com/bliki/IntegrationTest.html)

## Mutation testing as a suite-quality probe

Coverage % does not tell you whether assertions actually catch bugs. Mutation testing does.

**Mechanism.** Inject small AST mutations (operator swap, condition flip, value swap); run the suite; surviving mutants reveal weak assertions.

```
mutation_score = killed / (killed + surviving - equivalent)
pr_gate := mutation_score(diff_lines) >= 80
```

**Cost.** Expensive. Scope to critical modules and changed lines per PR; full nightly.

Evidence: *"Mutation testing is a powerful technique that helps you find flaws in your test suite, not just in your production code… A mutation score of 100% signifies that the test was comprehensive."* (https://testrigor.com/blog/understanding-mutation-testing-a-comprehensive-guide)

## Risk-based regression packs

Regression suites bloat over time and unmaintained tests become noise.

- Group tests into packs keyed by risk area (payment, auth, search…).
- After every release: **add** packs for new functionality, **remove** packs for retired functionality, **merge** overlapping packs.
- Use change-impact analysis to pick the right pack per change.

Evidence: *"Refine the packs after every release… Add… Remove… Merge."* (https://sgbi.us/10-best-practices-for-effective-regression-testing) *"Tailor strategies to bug fix / major enhancement / overhaul."* (https://katalon.com/resources-center/blog/risk-based-approach-for-regression-testing)

## Two-tier accessibility coverage

WCAG conformance cannot be fully automated. Tools catch structural defects; humans catch semantics.

- **Automated (every PR):** axe-style scanners flag missing alt text, label, contrast, focus.
- **Manual (quarterly):** human audit + assistive-tech (screen reader, switch device) testing for context and meaning.

Evidence: *"Automated accessibility testing cannot detect all issues… Tools can flag that an image has alt text, but only a person can judge if that text actually describes the image well."* (https://www.levelaccess.com/blog/automated-accessibility-testing-a-practical-guide-to-wcag-coverage)

## Real-system gates vs mocks (reconciled)

The corpus has two streams that look opposed:

**Pro real systems at the gate.** Contract tests let you avoid an integration env yet keep the boundary honest (pactflow.io). Kent C. Dodds: *"Write tests. Not too many. Mostly integration."* E2E should be *"as little as possible mocking in place"* (https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications). Martin Fowler considers a *"mature QA in Production capability"* credible enough that *"there may be no end-to-end system testing done at all"* — but only when prod observability is strong (https://martinfowler.com/articles/practical-test-pyramid.html).

**Pro mocks at the unit.** Harness explicitly says mock external dependencies and clocks for determinism (harness.io). Jest mocking guide argues for mocking via DI to *"force our unit under test to flow through different logical branches of the code – all without relying on any external systems"* (https://devblogs.microsoft.com/ise/jest-mocking-best-practices).

**Reconciliation:**

- Mocks belong at the **unit boundary** (own code, own pure logic).
- Real systems belong at the **integration and pre-prod gates**.
- **Contract tests** bridge the two so you never rely on a fake provider that diverges from reality.

The user's stance — *"final validation must rely on real integration and end-to-end tests"* — is the right one. Mocks isolate; they do not validate.

## Hot takes worth adopting

- **Treat retries as a smell, not a strategy.** *"Developers thought they 'fixed' the flaky tests by increasing some time values… these time values actually have no effect."* (https://trunk.io/blog/the-ultimate-guide-to-flaky-tests)
- **Flakiness can be a real bug.** *"Is this flakiness testing something that could happen in production… The flakiness is a signal that users could see this timing problem. Make the code work."* (https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline)
- **Coverage % is the wrong dashboard; mutation score is closer.** Surviving mutants reveal weak assertions even at 100% line coverage (https://testrigor.com/blog/understanding-mutation-testing-a-comprehensive-guide).
- **E2E suites scale exponentially, contracts scale linearly.** Collapse E2E to smoke + canary; push semantics into contracts (https://pactflow.io/blog/contract-testing-vs-integration-testing).
- **Quarantine + named owner beats both ignoring and deleting.** Keeps coverage running while protecting merge signal.
- **Vendor "AI-powered flake detection" is mostly marketing.** Underlying detection only needs rerun-on-same-SHA history. Teams can implement this without buying a product.

## Sources

- Trunk.io — *The Ultimate Guide to Flaky Tests* — https://trunk.io/blog/the-ultimate-guide-to-flaky-tests
- Harness — *Flaky Tests: The Quiet Killer* — https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline
- Thinksys — *How to reduce flaky tests* — https://thinksys.com/qa-testing/how-to-reduce-flaky-tests-automation-frameworks
- Gradle — *A Pragmatist's Guide to Flaky Test Management* — https://gradle.com/blog/a-pragmatists-guide-to-flaky-test-management
- AWS — *Testing stages in CI/CD* — https://docs.aws.amazon.com/whitepapers/latest/practicing-continuous-integration-continuous-delivery/testing-stages-in-continuous-integration-and-continuous-delivery.html
- Pactflow — *Contract Testing vs Integration Testing* — https://pactflow.io/blog/contract-testing-vs-integration-testing
- Pactflow — *What is Consumer-Driven Contract Testing* — https://pactflow.io/what-is-consumer-driven-contract-testing
- Martin Fowler — *IntegrationTest* — https://martinfowler.com/bliki/IntegrationTest.html
- testRigor — *Understanding Mutation Testing* — https://testrigor.com/blog/understanding-mutation-testing-a-comprehensive-guide
- Sgbi — *10 Best Practices for Effective Regression Testing* — https://sgbi.us/10-best-practices-for-effective-regression-testing
- Katalon — *Risk-Based Approach for Regression Testing* — https://katalon.com/resources-center/blog/risk-based-approach-for-regression-testing
- Level Access — *Automated Accessibility Testing* — https://www.levelaccess.com/blog/automated-accessibility-testing-a-practical-guide-to-wcag-coverage
- Mabl — *Overview of Automated Accessibility Testing* — https://www.mabl.com/articles/overview-of-automated-accessibility-testing
- Microsoft — *Jest Mocking Best Practices* — https://devblogs.microsoft.com/ise/jest-mocking-best-practices
- Martin Fowler — *Practical Test Pyramid* — https://martinfowler.com/articles/practical-test-pyramid.html
- Kent C. Dodds — *Testing Trophy* — https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications
- Go Wiki — *Table-Driven Tests* — https://go.dev/wiki/TableDrivenTests
