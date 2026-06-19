---
name: testing-boss
description: Comprehensive testing doctrine for software and AI systems — covers positive patterns, anti-patterns, gates for coding agents writing tests, CI discipline, and an LLM/agent evaluation primer. Use when authoring or reviewing tests, adding mocks, deciding test placement, generating tests via agents, debugging flaky CI, designing eval suites for LLM features, or rebuilding a brittle test suite. Contains 12 positive patterns (selector hierarchy, table-driven, builders, real-system gates), 25 anti-patterns across Brittleness, Flakiness, Mock-misuse, Process, and AI-specific families, 7 mandatory gates for agents writing tests, flaky-test taxonomy with quarantine workflow, contract / property / mutation testing patterns, and an oracle-ladder primer for LLM-as-judge and agent eval. Language-agnostic — pseudo-code only. Don't use for general code review, library-specific debugging unrelated to tests, non-testing CI pipeline design, or production observability.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Testing Boss

A consolidated doctrine for writing tests that *reveal bugs*, not just pass — for human-authored code, AI-generated code, LLM-powered features, and the CI that gates them all.

The cardinal premise: **tests exist to expose defects, not to keep CI green.** A test that fails has done its job. A test that passes for the wrong reason is worse than no test.

This skill collapses the old `test-antipatterns` skill plus a much larger corpus on test placement, framework idioms, flaky-test discipline, AI-agent test generation, and LLM/agent evaluation into one self-contained body of practice. Examples are language-agnostic pseudo-code so the doctrine transfers to any stack.

## Iron Laws

```
1. Test the behavior, never the mock.
2. Push every test to the lowest layer that can detect the failure.
3. When a test fails, fix production first — change the test only after writing why.
4. Real systems gate the merge. Mocks isolate; they do not validate.
5. Coverage is a flashlight. Mutation score is a quality probe. Neither is a target.
6. No test-only methods, branches, or flags leak into production code.
```

These six laws subsume every named anti-pattern in this skill. When two of them disagree, the lower-numbered one wins.

## Required Reading Router

Match the task to the row. Read the listed file(s) **in full before** producing output. The inline content in this SKILL.md is a tripwire, not the contract.

| Task                                                        | MUST read                                                                  |
| ----------------------------------------------------------- | -------------------------------------------------------------------------- |
| Deciding where a new test belongs (layer, file, owner)      | `references/foundations.md`                                                |
| Writing a new test (any layer, any framework)               | `references/patterns.md`                                                   |
| Reviewing a test, smelling a problem, or fixing a brittle suite | `references/antipatterns.md`                                           |
| Generating tests via a coding agent (Claude Code, Codex, Cursor) | `references/ai-writes-tests.md` + `references/antipatterns.md`         |
| Triaging flaky tests, designing CI gates, or picking contract/property/mutation patterns | `references/ci-automation.md`              |
| Designing evals for LLM/agent systems (RAG, tool use, prompt regression) | `references/llm-eval.md`                                       |
| Looking up the original source for any claim in this skill  | `references/sources.md`                                                    |

## Reference Index

- **`references/foundations.md`** — placement doctrine (invariant + owning layer), pyramid vs trophy debate resolution, risk-based prioritization, coverage philosophy, test-boundary contracts.
- **`references/patterns.md`** — 12 cross-framework positive patterns with agnostic pseudo-code: selector hierarchy, condition-based waits, per-test isolation, table-driven, builders/factories, behavior-first assertions, boundary-only mocking.
- **`references/antipatterns.md`** — 25 anti-patterns across five families (Brittleness, Flakiness, Mock misuse, Process, AI-specific). Each entry: violation, why wrong, fix, gate question, evidence URL.
- **`references/ai-writes-tests.md`** — seven mandatory gates with verbatim prompt blocks for any agent that generates tests: invariant first, owning layer, real execution, failure→fix production, no snapshot without contract, no assertion on self-set mock, negative companion.
- **`references/ci-automation.md`** — flaky-test taxonomy, quarantine-plus-owner workflow, CI stage pyramid, contract / property / mutation / accessibility testing patterns, deterministic test architecture.
- **`references/llm-eval.md`** — eval-driven development primer, oracle ladder, LLM-as-judge biases and calibration, RAG metrics, agent trajectory vs outcome eval, benchmark pitfalls.
- **`references/sources.md`** — consolidated bibliography (all URLs grouped by axis) for citation and audit.

## Decide before the first line of test code

Most bad tests are placement failures, not assertion failures. A test in the wrong layer is brittle, slow, and duplicates work — or worse, it locks in implementation under the disguise of correctness.

Gist tripwires:

- Name the invariant in one sentence before opening any test file. If the sentence is fuzzy, the invariant is not clear enough to test.
- Place the test at the **lowest layer** that can fail when the invariant breaks. A higher-layer test is justified only when the invariant requires real integration the lower layer cannot prove.
- Reject the test entirely when (likelihood-of-bug × blast-radius) is below the threshold worth ten minutes of maintenance. Not every line deserves a test.

**STOP. Read `references/foundations.md` in full before placing a new test, splitting a test across layers, debating pyramid vs trophy, or arguing about coverage targets.** That file contains the placement decision tree, the explicit pyramid/trophy reconciliation, the test-boundary contract template, and the risk-based filter. The three tripwires above are detection cues, not the contract.

## Pattern catalog (write tests that survive refactor)

Twelve patterns recur across Playwright, Testing Library, Cypress, Jest, pytest, Go testing, and Pact. The framework is evidence; the principle is universal.

Named patterns (one-liners — full pseudo-code in the reference):

1. Query by behavior and accessible role, never by CSS selector or DOM index.
2. Selector hierarchy: role → label → text → test-id → structural. Stop at the highest rung that disambiguates.
3. Wait on observable conditions, never on wall-clock sleeps.
4. Each test is independent and order-free; setup beats teardown.
5. One behavior per test, but as many assertions as that behavior needs.
6. Test names read as specifications: `should <outcome> when <condition> given <state>`.
7. Table-driven / parameterized when only the inputs vary.
8. Build test data via factories or builders; literal blobs only for the field under test.
9. Mock at boundaries you do not control; real wiring for what you own.
10. Real systems gate the final merge; contract tests bridge unit and E2E.
11. Mutation score, not coverage percentage, measures suite strength.
12. Page Object Model is a tool, not a religion — collapse it for small suites.

**STOP. Read `references/patterns.md` in full before writing any non-trivial test, choosing a selector strategy, designing test data, or deciding what to mock.** That file contains the pseudo-code, the cross-framework evidence, and the explicit "when to break this rule" carve-out for each pattern. The twelve one-liners above are a vocabulary index, not the contract — the operational rule for each pattern lives only in the reference.

## Anti-pattern families (do not do these)

Twenty-five anti-patterns cluster into five families. The top seven (bolded below) cause the most damage in modern codebases — especially when AI agents write the tests.

**Brittleness** — tests bound to internals.
1. **Brittle/implementation-detail selectors.**
2. Testing internal structure instead of observable behavior.
3. Testing private methods directly.
4. Snapshot-as-test (a snapshot replacing real assertions).
5. Vague existence assertions (`.should('exist')`, `toBeTruthy`).
6. Action without assertion.

**Flakiness** — tests that randomize their own verdicts.
7. **Static `sleep` / fixed-timeout waits.**
8. **Test order dependency / hidden shared state.**
9. Non-deterministic inputs (real clock, RNG, locale).

**Mock misuse** — tests that test the test setup.
10. **Asserting the mock exists.** *(absorbed from the previous `test-antipatterns` skill)*
11. Mock drift (mock no longer matches real API).
12. Over-mocking child components.
13. Incomplete mocks (missing fields the system consumes downstream).
14. Mocking at the wrong level (mocks slow-and-safe methods test logic depends on).

**Process** — pathologies of the team and the suite over time.
15. **Coverage as a vanity metric.**
16. Happy-path-only coverage.
17. Eternal `beforeAll` / shared setup that hides dependencies.
18. Cleanup in `afterEach` (use `beforeEach` instead).
19. Magic strings and logic in tests.
20. Testing against third-party sites you do not control.
21. Quarantine-as-cemetery (skip without owner or deadline).
22. **Retry-as-fix (auto-retry hiding real bugs).**
23. Duplicate tests across pyramid layers.
24. Weakening tests to make them pass.
25. **Mock-driven confidence** (the test sets up a mock and then asserts on its own setup).

The old `test-antipatterns` skill covered five of these (Asserting the mock exists, Test-only methods in production, Mocking without understanding, Incomplete mocks, Integration tests as afterthought). All five survive here — folded into Mock misuse and Process — and the framing question **"Are we testing the behavior of a mock?"** still applies.

**STOP. Read `references/antipatterns.md` in full when reviewing any test, debugging a flaky suite, refactoring tests after a refactor broke them, evaluating AI-generated tests, or deciding whether to delete or rescue a struggling test file.** That file has the full 25-entry catalog with violation pattern, why-wrong, fix, gate question, and citation per entry. The family list above is a topic index, not the contract — flagging a pattern by family is not the same as knowing the fix.

## AI agents writing tests

Coding agents will mock everything to green by default. They produce long, linear tests with high assertion density and no edge cases — and they will happily patch the test instead of fixing the code. The skill ships seven gates to block that.

Gate names (the prompt blocks live in the reference):

1. **Invariant first** — agent prints `INVARIANT: …`, `OWNING_LAYER: …`, `EXISTING_SUITE: …` before any test code.
2. **Owning layer** — extend an existing suite; reject new files without a named invariant.
3. **Real execution** — every new test must run against a real DB / route / external integration at least once.
4. **Failure → fix production** — on red, the next tool call reads the production code, not the test.
5. **No snapshot without contract** — classify the artifact as `PRODUCT_CONTRACT` or `IMPLEMENTATION_DETAIL`; the latter forbids snapshots.
6. **No assertion on self-set mock** — cannot assert on a value the same test body wrote into a mock.
7. **Negative companion** — every positive assertion ships with a negative test for invalid input or failure mode.

**STOP. Read `references/ai-writes-tests.md` in full before letting any agent generate, modify, or "fix" tests in this repository.** That file contains the verbatim prompt blocks to paste into CLAUDE.md, the failure-protocol template, the mock-budget rule, and the evidence base (Anthropic's eval guide, the Yoshimoto study on AI test smells, the Stanford vibe-coding vulnerability finding). The seven gate names above are not enforceable on their own — the agent-runnable prompt block lives only in the reference.

## CI & flaky discipline

Tests that pass intermittently are worse than tests that always fail. They train the team to ignore signal.

Gist tripwires:

- Quarantine a flaky test the same hour it is detected. Assign a named human owner within 24 hours with a fix-by date. No anonymous quarantines.
- Track `flaky_rate` as a first-class operational metric. SLO < 1–2%; alert at > 5%. Retry without telemetry is debt accrual, not stability.
- Real systems at the final gate. Mock at unit; contract-test the boundary; real DB / queue / route at integration; near-zero mocks at E2E.

**STOP. Read `references/ci-automation.md` in full before designing CI stages, introducing retries, choosing between integration and contract tests, adding property or mutation testing, or building a regression pack.** That file contains the flaky taxonomy (Async 45%, Concurrency 20%, Order 12%, …), the quarantine workflow, the CI stage pyramid, and the contract/property/mutation/accessibility patterns. The three tripwires above are detection cues, not the contract.

## LLM and agent evaluation (Part 6, enxuta)

Testing an LLM feature is conventional testing with one twist: the oracle is probabilistic. Everything else — invariants, regression, traceability, real-system validation, repair-production — applies harder, not less.

Gist tripwires:

- Start small. Twenty unambiguous tasks drawn from real failures beat two hundred synthetic ones.
- Climb the oracle ladder: exact / schema / outcome-state checks before LLM-as-judge before human review. Use the cheapest oracle that catches the failure.
- LLM-as-judge needs calibration. Validate against humans (target ≥ 0.80 Spearman) before trusting any judge in CI. Always use a different model than the system under test.
- Agents need outcome checks. Trajectory grading punishes valid creativity; outcome-only grading misses ghost actions where the transcript claims success and nothing changed.

**STOP. Read `references/llm-eval.md` in full before designing an eval suite, introducing LLM-as-judge into a pipeline, debating SWE-bench / τ-bench numbers, building RAG faithfulness checks, or rolling out trace-based observability.** That file contains the oracle ladder in full, the LLM-as-judge bias list, the RAG metric decomposition, agent trajectory-vs-outcome guidance, and the benchmark-pitfalls evidence (Anthropic eval guide, the 2507.02825 benchmark-validity paper, Galileo and Braintrust frameworks). The four tripwires above are detection cues, not the contract.

## Red flags (cross-cutting)

These signals should always trigger "stop and think" — no matter the layer or framework.

- Mock setup is larger than the test logic.
- Test breaks when an internal method is renamed (not a public contract).
- Removing the assertion body leaves the test still green.
- Test fails when run with `.only` in isolation.
- `sleep`, `Thread.sleep`, or `cy.wait(<number>)` appears anywhere.
- Selector contains a CSS class, an index, or `xpath`.
- Test asserts a third-party site is reachable.
- Snapshot diffs are accepted in code review without reading them.
- Coverage percentage is the only quoted quality metric.
- Failing tests are auto-retried until green; nobody investigates.
- Skipped or quarantined tests have no named owner and no fix-by date.
- Test depends on `new Date()`, `Math.random()`, or system locale.
- `afterEach` resets the database (move it to `beforeEach`).
- An AI-written test has six+ assertions and zero edge cases.
- The phrase "I'll mock this to be safe" appears anywhere in the diff.

## When NOT to use this skill

- General code review unrelated to tests — use a code-review skill.
- Library-specific debugging where the test is just the reproduction — use the library's own debugging skill.
- Non-testing CI pipeline design (deploys, artifact promotion, secrets management).
- Production observability and alerting design — those are runtime concerns, not test concerns.
- Single-line typo fixes in existing tests — the doctrine is for non-trivial work.

## Bottom line

```
A test that cannot fail is decorative. A test that fails for the wrong
reason is misleading. Build tests that fail for exactly one reason —
the reason the invariant was violated — and trust them when they do.

Mocks isolate. Real systems validate. Coverage shines a light. Mutation
score grades the suite. Agents will reach for the mock and the snapshot;
the gates here make them put both down.

Tests reveal bugs, not just pass.
```
