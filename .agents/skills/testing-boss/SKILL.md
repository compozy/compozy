---
name: testing-boss
description: Testing doctrine for tests that reveal bugs instead of passing for the wrong reason — spanning software and LLM/AI systems. Use when authoring or reviewing tests, adding a mock, deciding where a test belongs, letting a coding agent generate tests, triaging flaky CI, designing an eval suite for an LLM/agent feature, or rebuilding a brittle suite. Not for general code review, library debugging unrelated to tests, CI pipeline design beyond tests, or production observability.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# Testing Boss

Tests exist to expose defects, not to keep CI green. A test that fails has done its job; a test that passes for the wrong reason is worse than none. The doctrine spans human and AI-generated code, LLM/agent features, and the CI that gates them; its body lives in `references/` as language-agnostic pseudo-code.

## Iron Laws

Apply every law that bears on the change under test. They subsume every anti-pattern named in the references; when two disagree, the lower-numbered one wins.

```
1. Test the behavior, never the mock.
2. Push every test to the lowest layer that can detect the failure.
3. When a test fails, fix production first — change the test only after writing why.
4. Real systems gate the merge. Mocks isolate; they do not validate.
5. Coverage is a flashlight. Mutation score is a quality probe. Neither is a target.
6. No test-only methods, branches, or flags leak into production code.
```

## Reference router

The Iron Laws are the always-loaded tripwire; each reference is the contract. Match the task, read the listed file(s) **in full** before producing output, and apply every gate, pattern, and principle in them that bears on the work.

| When you are… | Read in full |
| --- | --- |
| Deciding where a test belongs — layer, owner, boundary, or whether to write it at all | `references/foundations.md` |
| Writing a test at any layer — selectors, waits, test data, isolation, what to mock | `references/patterns.md` |
| Reviewing a test, smelling brittleness, or rebuilding a brittle suite | `references/antipatterns.md` |
| Letting a coding agent generate, modify, or "fix" tests | `references/ai-writes-tests.md` + `references/antipatterns.md` |
| Triaging flaky CI, designing gates, or choosing contract / property / mutation tests | `references/ci-automation.md` |
| Designing an eval for an LLM/agent feature — oracle ladder, LLM-as-judge, RAG, trajectory vs outcome | `references/llm-eval.md` |

Each reference ends with its own sources; `references/sources.md` is the consolidated bibliography for auditing any claim.
