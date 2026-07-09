# Flaky Test Triage

This reference defines vocabulary, diagnosis protocol, and quarantine policy for flaky tests encountered during an `agent-output-audit` run. It exists because retrying a failing test until it passes is the most common way real bugs reach production — and the `agent-output-audit` skill must not silently promote a flake to `PASS` via retry. It does **not** document how to fix each cause; that is a test-pattern concern.

## Vocabulary

- **flaky-suspect** — A test failed once, passed on retry without any code change. Awaiting confirmation runs. Cannot be promoted to `PASS` without further evidence.
- **quarantined** — Confirmed flaky after isolation runs. Isolated from the merge gate but still executed for monitoring. Requires a **named owner** (a person, not a team) and a **fix-by date**.
- **flake rate** — Percentage of tests in the canonical suite that produced inconsistent verdicts across runs in the current window.

## Cause classification

When labeling a flake event in the audit report, pick one category:

- `async-wait` — failure varies with timing of async I/O or DOM updates.
- `concurrency` — failure varies with parallel execution or shared mutable state.
- `order-dep` — failure depends on the order tests run (or the contents of `beforeAll`/`beforeEach` state).
- `external` — failure depends on an external resource (network, clock, FS, 3rd-party API).
- `non-determinism` — failure varies with intrinsic randomness (RNG without seed, LLM temperature, model output).
- `orphan-code` / `fragile-locator` — failure varies with DOM structure, selector instability, or dead code paths.

The category is recorded for triage; the fix belongs to the implementing engineer (and to test-pattern skills), not to this QA flow.

## Diagnosis protocol

When a test fails in baseline or in a re-run, do **not** classify it as pre-existing or new until this protocol completes.

1. **Isolate**: Run the single failing test 3 to 5 times on the same SHA, in a clean working tree, with no other tests scheduled. Record each outcome.
2. **Stress order**: If isolation passes ≥ 1 time, run the surrounding describe block in randomized order 3 to 5 times.
3. **Stress concurrency**: If the suite supports parallel mode, run the affected file with the project's parallelization flag.
4. **Bisect**: If the test is new or recently modified, `git bisect` to identify the first commit that introduced the flake.
5. **Classify** using the cause categories above. Record `Suspected Category` in the issue.

## Retry policy

- **PROHIBITED**: Promoting a `FAIL` to `PASS` because a single retry passed. This includes CI-level "retry on failure" features when their outcomes are not surfaced in the audit report.
- **Allowed**: Running the diagnosis protocol above and classifying the test as `flaky-suspect` or `quarantined`.
- **Required**: Every flake event (failure → retry → pass) is recorded in `audit-report.md` under `SUITE HEALTH SNAPSHOT` → `Flaky events this run` with test name, attempts, retry outcome, and suspected category.
- **Threshold**: `Flake rate >= 2%` in the canonical suite is a `FAIL` on the `Flaky rate <2%` Quality Gate. Above this, the QA run cannot conclude with an unconditional `PASS`.

## Quarantine workflow

When a `flaky-suspect` is confirmed flaky after the diagnosis protocol (i.e., the test cannot be made reliable within the QA window), move it to `quarantined`:

1. **Assign a named owner within 24 hours.** Not a team. A person. Without an owner, the test is auto-removed from the suite after one sprint.
2. **Set a fix-by date.** Maximum two sprints from quarantine. Past that date, the test is removed and a `BUG-<num>` is filed under `Type: Functional` with `Priority: P1`.
3. **Isolate from the merge gate.** Quarantined tests must still run in CI but their result must not block merges.
4. **Monitor**: Each `agent-output-audit` run reports the quarantine count in `SUITE HEALTH SNAPSHOT`.
5. **Re-entry gate**: A quarantined test returns to the main suite only after **10 consecutive clean runs** across CI and local. Document the 10 runs in the `BUG-<num>.md` resolution evidence.

## Compozy mode interaction

When the failing test is associated with a task whose `declared_status: completed` and the task lives under `.compozy/tasks/<slug>/`:

- If the failure is `flaky-suspect` on a **P0/P1** flow proving the task: degrade `qa_verdict` to `PARTIAL`, file `BUG-<num>.md` with Status `flaky-suspect`, and do **not** promote the task. Write the finding to `memory/qa-execution.md` → `Errors / Corrections` **before** flipping any frontmatter status (memory-precedes-status invariant).
- If the failure is `flaky-suspect` on a non-critical flow: record in the SUITE HEALTH SNAPSHOT, file `BUG-<num>.md`, but do not degrade the task verdict.
- A `flaky-on-completion` P0 task **never** passes the gate until the BUG is `resolved` or the flake is confirmed `invalid`.

## Sources

- Trunk — [The Ultimate Guide to Flaky Tests](https://trunk.io/blog/the-ultimate-guide-to-flaky-tests): retry-as-PASS is "kicking a can down the road"; ~45% of flakes are async-wait related per Luo et al.
- Gradle — [A Pragmatist's Guide to Flaky Test Management](https://gradle.com/blog/a-pragmatists-guide-to-flaky-test-management): record every retry; a stable test over flaky product code looks like a flaky test.
- Harness — [Flaky Tests: How to Find, Fix, and Prevent Them](https://www.harness.io/blog/flaky-tests-the-quiet-killer-of-productivity-in-your-ci-pipeline): healthy suites <1-2% flake, >5% indicates structural problems.
- Rainforest QA — [A Practical Guide to Reducing the Burden of Flaky Tests](https://www.rainforestqa.com/blog/flaky-tests): fix / re-run / disable / quarantine / delete spectrum.
- ThinkSys — [Reduce Flaky Tests: A Practical Guide for QA Teams](https://thinksys.com/qa-testing/how-to-reduce-flaky-tests-automation-frameworks): 75% of flakes fail in correlated clusters; pipeline consequences for unowned flakes.
