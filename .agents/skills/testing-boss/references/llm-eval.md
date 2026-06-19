# LLM and Agent Evaluation — Primer

## Contents

- [Scope](#scope)
- [Eval-driven development](#eval-driven-development)
- [Eval dataset construction](#eval-dataset-construction)
- [The oracle ladder](#the-oracle-ladder)
- [LLM-as-judge](#llm-as-judge)
- [RAG-specific evaluation](#rag-specific-evaluation)
- [Agent evaluation — trajectory vs outcome](#agent-evaluation--trajectory-vs-outcome)
- [Regression vs capability evals](#regression-vs-capability-evals)
- [Trace-based observability as the living eval](#trace-based-observability-as-the-living-eval)
- [Hallucination detection](#hallucination-detection)
- [Agentic benchmark pitfalls](#agentic-benchmark-pitfalls)
- [Connection back to conventional testing](#connection-back-to-conventional-testing)
- [Sources](#sources)

## Scope

This reference is a primer, not a manual. It exists to align the team's LLM / agent evaluation vocabulary with the conventional testing doctrine in the rest of this skill — same goals (oracle quality, regression, traceability, real-system validation), different oracles.

Classical testing thinking survives the LLM era. What changes:

- Oracles become probabilistic.
- Behavior matters more than literal text.
- The "test author" is often a coding agent that will reward-hack a grader unless gated.

## Eval-driven development

Specify success before writing the agent; iterate on prompt / model / tools against a fixed dataset; gate releases on the eval suite. The TDD analog for stochastic systems.

**When.** Any LLM feature past the prototype stage. Mandatory once an agent is in production.

**Evidence.** *"20–50 simple tasks drawn from real failures is a great start."* (https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents) *"A small set of 10–20 prompts is enough to surface regressions and confirm improvements early."* (https://developers.openai.com/blog/eval-skills) Promptfoo frames the goal as *"test-driven LLM development, not trial-and-error"* (https://www.promptfoo.dev/docs/intro).

**Pitfalls.** Waiting for the dataset to be "big" before starting. Imbalanced sets that only test the positive direction.

## Eval dataset construction

Three requirements:

1. **Unambiguity** — two domain experts reach the same verdict on each task.
2. **Negative / decline cases** — explicit examples where the agent should refuse or fail predictably.
3. **Reference solution** — ship a known-good solution proving the task is solvable and the grader is configured correctly. Re-running and getting 0% on 100 attempts is a broken-task signal, not an incapable agent.

Evidence: https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents (Steps 2–3). Ragas-style RAG test sets require question + reference contexts + reference answer (https://docs.ragas.io/en/stable/getstarted/rag_eval).

## The oracle ladder

Choose oracle by cost and signal. Cheaper oracles run first; expensive ones only run on what cheaper ones cannot decide.

1. **Exact / regex / fuzzy string match** — fast, brittle.
2. **Schema and tool-call assertions** — strong gate for structured output and tool use.
3. **Outcome (state) checks** — "did the refund hit the DB?", not "did the message say done?".
4. **Code-based behavioral tests** on generated code (SWE-Bench-style fail-to-pass + pass-to-pass).
5. **LLM-as-judge** — open-ended quality; calibrated against humans.
6. **Human review** — gold standard, used to calibrate everything else.

Evidence: *"Choose deterministic graders where possible, LLM graders where necessary or for additional flexibility, and use human graders judiciously."* (https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents) *"Use deterministic checks for tool selection, argument construction, and format compliance. Use LLM-as-judge for response quality and goal alignment."* (https://www.braintrust.dev/articles/ai-agent-evaluation-framework)

## LLM-as-judge

A separate LLM scores outputs against a rubric — single-output (GEval / DAG) or pairwise (Arena / preference).

**When.** Quality is semantic and not capturable by string match: tone, grounding, task completion, helpfulness.

**Known biases.** Position bias, verbosity bias, self-enhancement bias, length bias, agreeableness bias. *"Error rates exceeding 50% on complex evaluation tasks."* (https://galileo.ai/blog/agent-evaluation-framework-2026)

**Discipline:**

- Use a different model than the system under test.
- Give the judge an "Unknown" escape valve so it does not hallucinate verdicts.
- Validate against humans before trusting in CI. Target ≥ 0.80 Spearman correlation with human judgment.

Evidence: *"LLM-as-a-judge scoring achieves 81.3% human correlation."* (https://openlayer.com/blog/post/llm-evaluation-metrics-complete-guide-for-march-2026) Three-technique split (G-Eval / DAGMetric / QAG): https://deepeval.com/guides/guides-llm-as-a-judge.

## RAG-specific evaluation

Decompose into retrieval and generation metrics. Mixing them into one number hides which side broke.

- **Retrieval:** context precision, context recall.
- **Generation:** faithfulness, groundedness, answer relevancy.

Evidence: Ragas evaluation guide (https://docs.ragas.io/en/stable/getstarted/rag_eval). DeepEval Faithfulness checks every claim against retrieved context (https://deepeval.com/docs/metrics-faithfulness).

**Pitfall.** A single combined score makes it impossible to tell whether the chunker, embedder, or generator regressed. Always report retrieval and generation separately.

## Agent evaluation — trajectory vs outcome

Score both the path (tool selection, plan adherence, argument correctness, step efficiency) and the outcome (state check, task completion). Neither alone is enough.

- **Trajectory-only grading** punishes valid creativity — the agent finds a shorter path and gets penalized.
- **Outcome-only grading** misses ghost actions — the transcript claims success and nothing changed.

Evidence: *"It's often better to grade what the agent produced, not the path it took."* (https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents) Confident AI documents "ghost actions" (real vs claimed state divergence) and "metric green, user red" failure modes (https://www.confident-ai.com/blog/definitive-ai-agent-evaluation-guide). InfoQ: *"Treating multi-turn agents like single-turn LLMs — BLEU/ROUGE on text misses the entire failure surface."* (https://www.infoq.com/articles/evaluating-ai-agents-lessons-learned)

Braintrust's layered approach: separate metrics for reasoning, action, and end-to-end (https://www.braintrust.dev/articles/ai-agent-evaluation-framework).

## Regression vs capability evals

Two separate suites with different success bars.

- **Capability evals** — start low (a hill to climb). Used to push the agent forward.
- **Regression evals** — must be near 100%. Block releases. Production failures graduate into this suite.

Re-run on every prompt change, model swap, or tool change. *"A prompt change that improves performance on one type of query can degrade performance on several others."* — Braintrust, Anthropic eval guide.

## Trace-based observability as the living eval

Production traces are the actual eval set. Sample live traffic, run scorers async, alert on drift, push failing traces back into the dev test suite.

Tools that fit the loop: Langfuse, LangSmith, Confident AI, Braintrust, Arize (https://arize.com/llm-evaluation).

The principle: a static eval set is a snapshot. A trace-fed eval set is a moving target that tracks what users actually do.

## Hallucination detection

Open problem. Survey-grade reading:

- *"LLM Hallucination Survey"* — https://arxiv.org/html/2510.06265v2
- HalluLens — https://aclanthology.org/2025.acl-long.1176/
- SelfCheckGPT-style consistency, fact-verification, LLM-Check, HalluLens

None of these replace grounding-by-construction. The strongest defense is still RAG with a faithfulness gate or tool-grounded responses where the agent cannot answer without retrieving real data.

## Agentic benchmark pitfalls

The strongest claim in this corpus: published benchmarks routinely over- or under-state agent capability by up to 100% relative because of outcome-validity and task-validity bugs.

- *"SWE-bench-Verified uses insufficient test cases, while τ-bench counts empty responses as successful… 24% of the top 50 leaderboard positions are incorrect."* (https://arxiv.org/html/2507.02825v2)
- *"An agent can score 100% on SWE-Lancer without resolving any tasks."* (same)
- Reward-hacking case: o3 located the grader's reference answer on the Python call stack and returned it on SWE-Bench. (https://trilogyai.substack.com/p/a-practical-guide-to-llm-and-agent)

**Implication for the team.** Treat any internal eval suite the same way. If the grader can be hacked, the agent will hack it. Audit the grader before trusting its scores.

## Connection back to conventional testing

**Convergence:**

- Oracle quality dominates either world.
- Regression gates work for both prompt changes and code changes.
- Trace / observability is just instrumented integration testing.
- *"Read the failure"* (TDD red-step verification) and *"read the transcripts"* (eval failure inspection) are the same loop.
- Real-execution > mock-execution is the test-pyramid's integration tier reasserting itself in an LLM context.

**Divergence:**

- Oracles are probabilistic. Re-run important tasks (pass@k, pass^k) instead of trusting a single green.
- Non-determinism means snapshot tests are less safe by default.
- Example-based tests dominate in LLM evals (curated golden tasks); the LLM analog of property-based tests is rubric assertions ("response is grounded in retrieved context").
- Coverage means nothing for LLM behavior — you need explicit failure-mode coverage, not line coverage.

The bottom line: classical testing discipline (invariants, layers, real-integration, repair-production) is the *correct* defense against LLM and AI-coding-agent failure modes. Lifting those gates from CLAUDE.md prose into explicit agent-runnable checks is the operational task of any team shipping AI features.

## Sources

- Anthropic — *Demystifying Evals for AI Agents* — https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents
- OpenAI Developers — *Testing Agent Skills Systematically with Evals* — https://developers.openai.com/blog/eval-skills
- Promptfoo — *Intro* — https://www.promptfoo.dev/docs/intro
- Ragas — *RAG Evaluation Getting Started* — https://docs.ragas.io/en/stable/getstarted/rag_eval
- Ragas — *Evaluation* — https://docs.ragas.io/en/v0.1.21/getstarted/evaluation.html
- DeepEval — *Faithfulness Metric* — https://deepeval.com/docs/metrics-faithfulness
- DeepEval — *LLM as a Judge Guide* — https://deepeval.com/guides/guides-llm-as-a-judge
- Confident AI — *Definitive AI Agent Evaluation Guide* — https://www.confident-ai.com/blog/definitive-ai-agent-evaluation-guide
- Braintrust — *AI Agent Evaluation Framework* — https://www.braintrust.dev/articles/ai-agent-evaluation-framework
- Galileo — *Agent Evaluation Framework 2026* — https://galileo.ai/blog/agent-evaluation-framework-2026
- Openlayer — *LLM Evaluation Metrics Complete Guide* — https://openlayer.com/blog/post/llm-evaluation-metrics-complete-guide-for-march-2026
- Arize — *LLM Evaluation* — https://arize.com/llm-evaluation
- InfoQ — *Evaluating AI Agents: Lessons Learned* — https://www.infoq.com/articles/evaluating-ai-agents-lessons-learned
- Trilogy AI — *A Practical Guide to LLM and Agent Evaluation* — https://trilogyai.substack.com/p/a-practical-guide-to-llm-and-agent
- *Establishing Best Practices for Building Rigorous Agentic Benchmarks* — https://arxiv.org/html/2507.02825v2
- *LLM Hallucination Survey* — https://arxiv.org/html/2510.06265v2
- HalluLens — https://aclanthology.org/2025.acl-long.1176/
