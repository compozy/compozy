# Page Outline: SDK Testing Strategy
- **Purpose:** Provide guidance for unit, integration, and benchmark testing of SDK-based projects. Sources: tasks/prd-sdk/07-testing-strategy.md, _tests.md, go testing rules in `.cursor/rules/test-standards.mdc`.
- **Audience:** QA engineers and developers writing tests.
- **Prerequisites:** Familiarity with Go testing tools.
- **Key Sections:**
  1. Testing philosophy (context-first tests, avoiding globals) referencing 07-testing-strategy.md §"Principles".
  2. Unit testing builders (using fakes, BuildError assertions) referencing 07-testing-strategy.md §"Unit".
  3. Integration testing workflows (in-process runtime, temporal mocks) referencing 07-testing-strategy.md §"Integration" and 04-implementation-plan.md test harness notes.
  4. Performance/benchmarks (concurrency patterns) referencing 07-testing-strategy.md §"Benchmarks".
  5. Tooling & commands (gotestsum, make test) referencing _docs.md guidelines.
- **Cross-links:** Core testing guide, CLI diagnostic commands, troubleshooting page for test failures.
- **Examples Strategy:** Link to `sdk/examples/11_test_harness.go` and unit test snippets in `sdk/examples/tests/`.
- **Notes:** Emphasize using `t.Context()` and `logger.NewForTests()` per project rules.
