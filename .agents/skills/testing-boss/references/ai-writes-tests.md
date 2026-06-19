# AI Writes Tests — Seven Gates Plus Prompt Blocks

## Contents

- [Why this reference exists](#why-this-reference-exists)
- [Gate 1 — Invariant first](#gate-1--invariant-first)
- [Gate 2 — Owning layer](#gate-2--owning-layer)
- [Gate 3 — Real execution](#gate-3--real-execution)
- [Gate 4 — Failure means fix production](#gate-4--failure-means-fix-production)
- [Gate 5 — No snapshot without contract](#gate-5--no-snapshot-without-contract)
- [Gate 6 — No assertion on self-set mock](#gate-6--no-assertion-on-self-set-mock)
- [Gate 7 — Negative companion](#gate-7--negative-companion)
- [Combined prompt block — paste into CLAUDE.md or a skill](#combined-prompt-block--paste-into-claudemd-or-a-skill)
- [Behavioral protocols](#behavioral-protocols)
- [Evidence base](#evidence-base)
- [Sources](#sources)

## Why this reference exists

Coding agents default to mock-driven confidence. They write long, linear tests with high assertion density and no edge cases. On failure they patch the test, not the code. Without explicit gates, agent-generated tests look thorough and validate nothing.

The seven gates below convert the project's testing doctrine into agent-runnable checks. Each gate is enforceable as a prompt block, a failure protocol, or a review rule. None is optional.

The user's CLAUDE.md already names the cardinal premise:

> *"THE MAIN GOAL of writing tests is NOT just to make them pass — it is to discover potential and actual bugs in the system. When a test reveals unexpected behavior, a bug, or a regression, you MUST fix the production code instead of weakening the test or adjusting assertions to match broken behavior."*

These gates lift that prose into explicit pre-write checks.

## Gate 1 — Invariant first

The agent must name the invariant and owning layer *before* generating any test code. Without this gate, the agent free-associates and produces tests that pin implementation.

**Prompt block:**

```
Before writing any test code, output exactly:

INVARIANT: <one sentence; the property that must hold regardless of implementation>
OWNING_LAYER: <unit | service-integration | route-integration | e2e>
EXISTING_SUITE: <path to canonical suite that owns this layer, or NO_SUITE_FOUND>

IF NO_SUITE_FOUND: STOP. Ask the user where this test belongs.
```

**Why.** Anthropic's eval guide makes the equivalent point for evals: *"A good task is one where two domain experts would independently reach the same pass/fail verdict."* (https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents) An invariant that fits one sentence is testable; one that does not is not.

## Gate 2 — Owning layer

Default to extending an existing suite. Forbid creating new files unless the agent can name the invariant the new file owns.

**Operational rule.**

```
1. Read the canonical suite for OWNING_LAYER.
2. If the new invariant can extend that suite, append to it.
3. If a new file is required, the agent must:
   a. Name the file's invariant in one sentence.
   b. Explicitly state which existing suite this is NOT duplicating.
   c. Wait for human confirmation before creating.
```

**Why.** Yoshimoto et al. found AI-generated tests cluster into "isolated distributions" — tests in regions semantically far from any human test, exactly the symptom of off-layer placement (https://arxiv.org/html/2603.13724). Pinning a *gold-standard test file* in CLAUDE.md so the agent has a concrete style reference is the strongest known counter (https://nmn.gl/blog/cursor-ai-gold-files): *"Don't let AI guess what good code looks like. Show it explicitly."*

## Gate 3 — Real execution

Every agent-generated test must run against the real subsystem at least once before merge. *Real* has a strict definition:

| Layer                         | Real means                                                                        |
| ----------------------------- | --------------------------------------------------------------------------------- |
| Database integration          | Ephemeral container or test schema, not an in-memory mock.                        |
| HTTP / route integration      | Invocation through the actual handler chain, not handler unit-only.               |
| External API integration      | Replay / cassette is acceptable; pure mock is not the validation gate.            |
| Outcome assertion             | Query the resulting state, not just the response message ("ghost action" defense). |

**Prompt block:**

```
After generating the test:
1. State the integration boundary this test crosses.
2. If the answer is "none", the test is not sufficient — add an integration companion.
3. Run the test against the real boundary at least once. Paste the failure or success
   output before claiming the test is complete.
```

**Why.** *"An agent that works perfectly in a sandbox but silently misreports a failed refund in production hasn't passed any evaluation that counts."* (https://www.infoq.com/articles/evaluating-ai-agents-lessons-learned) Stanford vibe-coding study: developers using AI assistants were *"41% more likely to introduce security vulnerabilities when they trusted the generated code without manual verification."* (https://arxiv.org/abs/2211.03622, summarized in https://getautonoma.com/blog/vibe-coding-best-practices)

## Gate 4 — Failure means fix production

When a test fails, the default is to fix production, not the test.

**Failure protocol:**

```
TEST FAILURE PROTOCOL

1. Re-read the failing test and the System Under Test (SUT) in full.
2. Decide: is the test asserting the contract correctly?
   - If YES → propose a SUT change.
   - If NO  → before any test edit, write one paragraph titled
     "SUT_IS_CORRECT_BECAUSE:" with the justification.
3. Test edits require human confirmation. Never edit a test silently to
   make it green.
```

**Why.** Florian Bruniaux's TDD-with-Claude guide: *"Without explicit instruction, Claude will: 1. Write implementation code 2. Then write tests that pass against that implementation."* (https://github.com/FlorianBruniaux/claude-code-ultimate-guide/blob/main/guide/workflows/tdd-with-claude.md) Anthropic eval guide: *"You won't know if your graders are working well unless you read the transcripts and grades from many trials"* (https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents) — the same loop applies to test failures.

## Gate 5 — No snapshot without contract

When agents generate snapshots, they freeze whatever the code happens to output now. The snapshot becomes a tautology.

**Operational rule:**

```
Before generating a snapshot, classify the artifact:

  PRODUCT_CONTRACT — an artifact the product itself ships
                     (public API schema, generated SDK file, OpenAPI doc).
                     → Snapshot is allowed and recommended.

  IMPLEMENTATION_DETAIL — formatted UI text, error messages, CSS literal values,
                          internal JSON shapes, rendered DOM trees.
                          → Snapshot is FORBIDDEN. Use specific matchers instead.

If uncertain, the artifact is IMPLEMENTATION_DETAIL by default.
```

**Why.** The user's CLAUDE.md already names this: *"Do not add tests that only freeze implementation details, static prose, CSS literal values, generated output, snapshots, config shape, or file existence unless that artifact itself is the product contract."* Reinforced by Anthropic's caution about over-rigid grading (https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents).

## Gate 6 — No assertion on self-set mock

The single largest source of mock-driven confidence: the agent sets up a mock, calls the SUT, and asserts on the mock's return value. The test cannot fail unless the mock setup contradicts itself.

**Mock Budget Rule:**

```
- Unit tests at I/O boundaries may mock the boundary only.
- A test that mocks every collaborator named in the SUT MUST also have one
  integration-layer companion test that exercises the same path without mocks.
- A test cannot assert on a value the same test body wrote into a mock,
  unless an independent source-of-truth comparison is also asserted.
```

**Prompt fragment to enforce:**

```
For every assertion in this test, cite where the expected value comes from:
  (a) A constant from the SUT's source code, OR
  (b) A documented external contract (URL or spec), OR
  (c) A computation the SUT performs that the assertion re-derives.

Assertions on mock return values without one of (a)/(b)/(c) are forbidden.
```

**Why.** *"Mocks are tools to isolate, not things to test."* (https://martinfowler.com/articles/practical-test-pyramid.html) The original `test-antipatterns` skill captured this as the founding Iron Law.

## Gate 7 — Negative companion

Every agent-generated positive assertion ships with a negative counterpart.

**Operational rule:**

```
For every test that asserts the SUT accepts valid input, add a sibling test
that asserts the SUT rejects an invalid input or fails the expected failure
mode. The negative test must:

- use a different input from the positive test,
- assert a specific failure (error type, status code, message contains, etc.),
- NOT be a `should not throw` placeholder.

Coverage rule: if deleting the negative test still leaves the suite green,
the negative test was not protecting an invariant.
```

**Why.** *"Evaluating only happy paths: If your task suite mostly contains well-formed inputs that the agent handles easily, you'll get high scores that don't reflect real-world performance. Deliberate adversarial testing is not optional."* (https://www.mindstudio.ai/blog/ai-agent-custom-benchmarks-evaluation) Yoshimoto et al. quantify the AI bias toward happy paths (https://arxiv.org/html/2603.13724).

## Combined prompt block — paste into CLAUDE.md or a skill

Copy the block below into the test-authoring section of a project's CLAUDE.md, AGENTS.md, or any agent skill that generates tests:

```
[TESTING GATES — apply in order before any test code]

GATE 1 — INVARIANT FIRST
  Print INVARIANT, OWNING_LAYER, EXISTING_SUITE.
  If EXISTING_SUITE = NO_SUITE_FOUND, stop and ask the user.

GATE 2 — OWNING LAYER
  Default to extending the canonical suite. Justify any new file in writing.

GATE 3 — REAL EXECUTION
  Name the integration boundary this test crosses.
  If "none", add an integration companion. Run the real boundary at least once.

GATE 4 — FAILURE MEANS FIX PRODUCTION
  On red, read the SUT, propose a SUT change.
  Test edits require a written "SUT_IS_CORRECT_BECAUSE:" rationale
  and human confirmation.

GATE 5 — NO SNAPSHOT WITHOUT CONTRACT
  Classify the artifact PRODUCT_CONTRACT or IMPLEMENTATION_DETAIL.
  Snapshots are forbidden for IMPLEMENTATION_DETAIL.

GATE 6 — NO ASSERTION ON SELF-SET MOCK
  Cite the source of every expected value: constant in SUT, external contract,
  or re-derivable computation. No mock-return-value assertions otherwise.

GATE 7 — NEGATIVE COMPANION
  Every positive assertion ships with a negative test that asserts a specific
  failure mode. Delete the negative — does the positive still pass? If yes,
  the negative was hollow.
```

## Behavioral protocols

Beyond the seven gates, two recurring behaviors deserve their own rules.

### Read the failure verbatim before any edit

```
ON TEST FAILURE:
1. Print the full failure trace (not just the assertion line).
2. Read the SUT file referenced in the trace.
3. ONLY THEN propose any edit (test or SUT).
```

Mirrors Anthropic's *"read the transcripts"* doctrine (https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents).

### Treat AI-authored tests as junior PRs

```
Review checklist for every agent-generated test:
- Does Gate 1 output appear in the diff or PR description?
- Is the test placed in the canonical suite for its layer? (Gate 2)
- Does at least one integration companion exist? (Gate 3)
- Is the assertion roulette count ≤ 2 unrelated assertions? (Anti-pattern E.2)
- Does a negative companion exist and protect a specific invariant? (Gate 7)
- Mutation-test the file: does the suite still detect a defect?
```

## Evidence base

The corpus is unambiguous on three claims that justify the gates:

1. **AI agents produce longer, linear, assertion-dense tests with low cyclomatic complexity** — Yoshimoto et al., *Testing with AI Agents: An Empirical Study* (https://arxiv.org/html/2603.13724). Median assertions/test: 2.0 for AI vs 1.0 for human.

2. **AI-assisted code increases vulnerability density when authors trust it without verification** — Perry et al., Stanford / UIUC (https://arxiv.org/abs/2211.03622): *"41% more likely to introduce security vulnerabilities when they trusted the generated code without manual verification"*, summarized in https://getautonoma.com/blog/vibe-coding-best-practices.

3. **Agents will hack any grader they can read** — *"24% of the top 50 leaderboard positions are incorrect"* on benchmarks like SWE-bench-Verified and τ-bench (https://arxiv.org/html/2507.02825v2). Trilogy AI documents o3 reading the grader's reference answer off the Python call stack (https://trilogyai.substack.com/p/a-practical-guide-to-llm-and-agent). Apply the same skepticism to internal test suites — if the agent can hack the test, it will.

## Sources

- Yoshimoto et al. — *Testing with AI Agents: An Empirical Study* — https://arxiv.org/html/2603.13724
- Anthropic — *Demystifying Evals for AI Agents* — https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents
- Stanford / UIUC — *Do Users Write More Insecure Code with AI Assistants?* — https://arxiv.org/abs/2211.03622
- Autonoma — *Vibe Coding Best Practices* — https://getautonoma.com/blog/vibe-coding-best-practices
- Florian Bruniaux — *TDD with Claude* — https://github.com/FlorianBruniaux/claude-code-ultimate-guide/blob/main/guide/workflows/tdd-with-claude.md
- nmn.gl — *Cursor AI Gold Files* — https://nmn.gl/blog/cursor-ai-gold-files
- InfoQ — *Evaluating AI Agents: Lessons Learned* — https://www.infoq.com/articles/evaluating-ai-agents-lessons-learned
- Mindstudio — *AI Agent Custom Benchmarks Evaluation* — https://www.mindstudio.ai/blog/ai-agent-custom-benchmarks-evaluation
- Martin Fowler — *The Practical Test Pyramid* — https://martinfowler.com/articles/practical-test-pyramid.html
- Confident AI — *Definitive AI Agent Evaluation Guide* — https://www.confident-ai.com/blog/definitive-ai-agent-evaluation-guide
- Trilogy AI — *A Practical Guide to LLM and Agent Evaluation* — https://trilogyai.substack.com/p/a-practical-guide-to-llm-and-agent
- *Establishing Best Practices for Building Rigorous Agentic Benchmarks* — https://arxiv.org/html/2507.02825v2
