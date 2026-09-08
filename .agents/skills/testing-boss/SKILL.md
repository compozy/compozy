---
name: testing-boss
description: "Author or review software tests and LLM/agent evals; choose test placement and mocks, diagnose flaky CI, or repair brittle suites. Excludes unrelated code review, debugging, CI design, and production observability."
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# Testing Boss

Choose tests that can expose the changed behavior's failure. Use the existing suite at the lowest layer that can observe the invariant; reuse fixtures and utilities before creating new files.

- Assert observable results, not mock behavior or implementation structure. Keep mocks at unit-test I/O boundaries; use real integration evidence for integration claims.
- When a test exposes a regression, fix production code. Change an incorrect test only with evidence that its contract is wrong or intentionally changed; preserve valid coverage.
- Existing tests or stronger build/codegen/link/render checks may already own the invariant. Do not add prose, CSS, generated-output, configuration, or snapshot tests without a real artifact contract and a reason the owning gate is insufficient.
- Coverage and mutation scores help find blind spots; they do not replace a behavioral oracle. Do not add production branches or methods solely for tests.
- Run affected checks, then the project's delivery gates. Reuse results for unchanged inputs and expand only for a failure, relevant edit, or unresolved risk. Match regression reproduction to the failure; avoid a second ceremony when red/green evidence already exists.

## References by Question

Read the relevant section, including its contract dependencies, when the question arises; test authorship by an agent does not itself require every reference.

| Question | Reference |
| --- | --- |
| Which layer or suite owns the invariant? | `references/foundations.md` |
| How should selectors, waits, data, or mocks work? | `references/patterns.md` |
| Why is a test brittle or passing for the wrong reason? | `references/antipatterns.md` |
| How should agent-generated tests be evaluated? | `references/ai-writes-tests.md` |
| How should flakiness or contract/property/mutation checks be handled? | `references/ci-automation.md` |
| How should an LLM/agent outcome be evaluated? | `references/llm-eval.md` |

`references/sources.md` holds supporting sources. Completion evidence can be a concise command/result summary; no separate report is needed unless the task's artifact contract requires one.
