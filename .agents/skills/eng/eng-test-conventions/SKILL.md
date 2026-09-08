---
name: eng-test-conventions
description: >-
  Go test-shape discipline for Compozy. Use when writing or editing *_test.go under
  cmd or internal after test placement is justified. Do not use for non-Go
  tests, fixture-only changes, or as a replacement for eng-consolidate-test-suites.
trigger: implicit
---

# Compozy Test Conventions

Name the invariant, owning layer, and existing suite before changing coverage; reuse that working note. Use `eng-consolidate-test-suites` when placement is unclear, including a proposed new file with no obvious owner. No test is needed merely to raise coverage.

Read the nearby canonical suite and relevant sections of `references/test-shape-rules.md` for subtests, parallelism, errors/assertions, interfaces, tags, integration behavior, mocks, helpers, and race/cgo.

- Co-ship affected ACP/E2E fixtures, typed matchers, and generated expectations with runtime-contract changes. Exercise real SQLite or production-like process boundaries where the owning layer requires them.
- Repair production regressions without weakening tests. If an assertion itself contradicts the established contract, substantiate that mismatch before changing it.
- Keep touched tests in the canonical subtest shape; do not rewrite unrelated suites. Transitive `t.Setenv` use keeps the enclosing test serial.

For changed Go test files, run the read-only heuristic checker:

```bash
python3 .agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py <file_path>
```

Fix real findings; document a demonstrated false positive without weakening the convention. Run the affected repository race-enabled suite with `CGO_ENABLED=1` and the owning lint lane, reusing current evidence when those inputs are unchanged. Root `make gate` and PR CI apply once at the enclosing delivery stage; a local test change does not require opening a PR.
