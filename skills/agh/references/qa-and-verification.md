# QA And Verification

## Contents

- Test decision
- Existing-suite preference
- Real-system confidence
- Command selection
- Final verification
- QA labs
- Scenario QA references

## Test Decision

Every AGH task needs a test decision, not automatic new tests. Before adding or changing tests, state:

- invariant
- owning layer
- canonical suite
- verification command

If no existing suite owns the invariant, create the smallest appropriate suite and explain why. If existing gates already prove the behavior, record a no-new-test rationale.

## Existing-Suite Preference

Prefer updating existing suites. Avoid duplicate regression coverage across layers unless each layer proves a distinct failure mode.

Do not add tests that only freeze implementation details, static prose, generated output, file existence, snapshots, CSS literal values, or config shape unless the artifact itself is the product contract.

Common static-artifact rejections include Storybook setup/config/bootstrap, CI workflow/action YAML, Mage/Make/package-script plumbing, generic config/file existence, generated-output drift, snapshots/goldens, and coverage-padding. Allow these only when the artifact itself is the product/operator/agent contract and no stronger gate owns the invariant.

## Real-System Confidence

Mocks are acceptable at boundaries not owned by the test, but final validation should exercise real wiring for the behavior being changed. Contract tests bridge unit tests and end-to-end tests. Real systems gate release confidence.

For task, session, network, or provider work, prefer tests that prove daemon behavior through the public surface or owning service, not prompt text alone.

## Command Selection

Match command scope to the claim:

- Narrow Go package behavior: targeted go test ./path.
- Bun workspace behavior: Turbo-backed bunx turbo run test --filter=...
- Site docs type/build behavior: site source generation and Turbo-backed test/typecheck.
- Whole task completion: make verify unless the change is docs-only and explicitly exempt.

Do not use package-local Bun test commands as validation evidence when repo rules require Turbo.

## Final Verification

Before claiming completion:

1. Identify what command proves the claim.
2. Run it fresh after the final change.
3. Read exit code and output.
4. Report failures honestly and fix root causes.
5. Re-run after fixes.

Passing tests are evidence only for what they cover. Audit the original requirements against artifacts before declaring the task done.

## QA Labs

For release or scenario QA, use isolated AGH homes, daemon ports, and bridge sockets. Reuse a QA lab only when continuing the same active QA session with a matching manifest. Fresh independent QA passes should use fresh labs.

Persist lab root, runtime home, base URL, and verification evidence when a QA pass creates a bootstrap manifest.

## Scenario QA References

Release-grade QA follows SD-005 in `docs/_memory/standing_directives.md`: `make verify` is necessary but not sufficient when the change affects real agent workflows, provider lanes, network/task orchestration, or UI surfaces.

Use the `real-scenario-qa` skill for release/scenario validation. It routes through `qa-report` and `qa-execution` for persona, journey, exploratory, and evidence capture.

When parallel QA or parallel agents are involved, apply `docs/_memory/lessons/L-009-concurrent-worktree-deadlock.md`: unique `AGH_HOME`, daemon port, and bridge socket per lab. When native providers are involved, apply `docs/_memory/lessons/L-016-native-provider-qa-home-policy.md`: isolate AGH runtime state, but preserve operator `HOME` for `native_cli` providers with `home_policy=operator` unless the scenario explicitly tests isolated provider-home behavior.
