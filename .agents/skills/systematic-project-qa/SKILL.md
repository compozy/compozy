---
name: systematic-project-qa
description: Executes full-project QA like a real user by discovering the repository verification contract, running build, lint, test, and startup commands, exercising core workflows end-to-end, creating realistic fixtures when needed, fixing root-cause regressions, and rerunning the full gate. Use when validating a branch, release candidate, migration, refactor, or risky commit. Do not use for static code review only, one-off unit test edits, or architecture brainstorming without execution.
---

# Systematic Project QA

## Procedures

**Step 1: Discover the Repository QA Contract**
1. Read root instructions, repository docs, and CI/build files before running commands.
2. Execute `python3 scripts/discover-project-contract.py --root .` to surface candidate install, verify, build, test, lint, and start commands.
3. Prefer repository-defined umbrella commands such as `make verify`, `just verify`, or CI entrypoints over language-default commands.
4. Read `references/project-signals.md` when command ownership is ambiguous or when multiple ecosystems are present.
5. Identify the changed surface and the regression-critical surface before choosing scenarios.
6. Choose a QA artifact location using repository conventions. If the repository has no QA artifact convention, store scratch artifacts under `/tmp/codex-qa-<slug>`.

**Step 2: Define the QA Scope**
1. Build a short execution matrix covering baseline verification, changed workflows, and unchanged business-critical workflows.
2. Read `references/checklist.md` and ensure every required category has a planned validation.
3. Prefer public entry points such as CLI commands, HTTP endpoints, browser flows, worker jobs, and documented setup commands over internal test helpers.
4. Create the smallest realistic fixture or fake project needed to exercise the workflow when the repository does not already include one.
5. Treat mocks as a local unit-test boundary only. Do not use mocks or stubs as final proof that a user flow works.

**Step 3: Establish the Baseline**
1. Install dependencies with the repository-preferred command before testing runtime flows.
2. Run the canonical verification gate once before scenario testing to establish baseline health.
3. If the baseline fails, read the first failing output carefully and determine whether it is pre-existing or introduced by current work before moving on.
4. Start services in the closest supported production-like mode and confirm readiness through observable signals such as health checks, startup logs, or successful handshakes.

**Step 4: Execute User-Like Flows**
1. Drive workflows through the same interfaces a real operator or user would use.
2. Capture the exact command, input, and observable result for each scenario.
3. Validate changed features first, then validate at least one regression-critical flow outside the changed surface.
4. Exercise live integrations when credentials and local prerequisites exist. When they do not, validate every reachable local boundary and record the blocked live step explicitly.
5. Re-run the scenario from a clean state when the first attempt leaves the environment ambiguous.

**Step 5: Diagnose and Fix Regressions**
1. Reproduce each failure consistently before proposing a fix.
2. Activate companion debugging and test-hygiene skills when available, especially root-cause debugging and anti-workaround guidance.
3. Add or update the narrowest regression test that proves the bug when the repository supports automated coverage for that surface.
4. Fix production code or real configuration at the source of the failure. Do not weaken tests to match broken behavior.
5. Re-run the narrow reproduction, the impacted scenario, and the baseline gate after each fix.
6. Use `assets/issue-template.md` when the user wants persisted issue files or when the repository already has a QA issue convention.

**Step 6: Verify the Final State**
1. Re-run the full repository verification gate from scratch after the last code change.
2. Re-run the most important user-like scenarios after the full gate passes.
3. Summarize the evidence using `assets/verification-report-template.md`.
4. Report blocked scenarios, missing credentials, or environment gaps with the exact command or prerequisite that stopped execution.
5. Do not claim completion without fresh verification evidence from the current state of the repository.

## Error Handling

* If command discovery returns multiple plausible gates, prefer the broadest repository-defined command and explain the tie-breaker.
* If no canonical verify command exists, read `references/project-signals.md`, choose the broadest safe install, lint, test, and build commands for the detected ecosystem, and state that assumption explicitly.
* If a required live dependency is unavailable, validate every local boundary that does not require the missing dependency and report the blocked live validation separately.
* If a workflow requires data or services absent from the repository, create the smallest realistic fixture outside the main source tree unless the repository has its own fixture convention.
* If a failure appears unrelated to the requested change, prove that with a clean reproduction before excluding it from the QA scope.
