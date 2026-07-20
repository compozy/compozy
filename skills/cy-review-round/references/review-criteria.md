# Review Criteria

## Severity Levels

### critical

Security flaws, crashes, data loss, undefined behavior, or race conditions.
Issues that could cause production incidents or compromise user data.

Examples: authentication bypass, SQL/command injection, nil pointer dereference
on a hot path, unbounded goroutine leak, writing sensitive data to logs.

### high

Bugs affecting correctness, performance bottlenecks visible to users, or
anti-patterns that significantly impair scalability, reliability, or usability.
These need fixing before merge.

Examples: logic error returning wrong results, O(n^2) loop over unbounded
input, missing transaction rollback, error silently swallowed in a critical
path, missing input validation at a system boundary.

### medium

Maintainability concerns, code smells, test coverage gaps, or non-idiomatic
patterns that degrade long-term health. Not blocking but should be addressed.

Examples: duplicated logic across packages, function exceeding 80 lines with
deep nesting, missing test for an error branch, context.Background() used
outside main, interface accepted but only one implementation exists.

### low

Minor improvements, documentation gaps, or naming suggestions. Optional
enhancements that improve clarity.

Examples: unclear variable name, missing godoc on an exported function,
redundant type conversion, slightly misleading comment.

## Evaluation Areas

### 1. Security

- Authentication and authorization flaws.
- Input validation gaps (injection, path traversal, XSS).
- Hardcoded secrets, tokens, or credentials.
- Cryptography misuse or insecure storage.
- Sensitive data exposure in logs or error messages.

### 2. Correctness

- Logic errors producing wrong results.
- Off-by-one and boundary condition bugs.
- Nil or null pointer dereferences.
- Unhandled error paths leading to silent failures.
- Incorrect type assertions or conversions.

### 3. Concurrency

- Race conditions and missing synchronization.
- Goroutine leaks (no shutdown path or context cancellation).
- Deadlock potential from lock ordering.
- Channel misuse (send on closed, unbuffered blocking).
- Missing `sync.WaitGroup` tracking for spawned goroutines.

### 4. Performance and Scalability

- Algorithmic complexity issues (O(n^2) where O(n) suffices).
- Resource leaks (file handles, HTTP bodies, database connections).
- Unbounded growth in slices, maps, or channels.
- Missing caching for repeated expensive operations.
- Blocking I/O on critical paths without timeout.

### 5. Error Handling

- Swallowed errors (assigned to `_` without justification).
- Missing error context (`fmt.Errorf("context: %w", err)`).
- `panic()` or `log.Fatal()` in library or handler code.
- Broad catch-all error handling masking specific failures.
- Incorrect use of `errors.Is()` or `errors.As()`.

### 6. Code Quality and Maintainability

- Readability issues (unclear naming, deeply nested logic).
- Code duplication across functions or packages.
- Overly complex functions that should be decomposed.
- Dead code or unused exports.
- Violations of project coding conventions.

### 7. Testing

- Missing tests for critical code paths.
- Tests that verify mocks instead of behavior.
- Flaky test patterns (time-dependent, order-dependent).
- Inadequate edge case and error path coverage.
- Missing `t.Parallel()` for independent subtests.

### 8. Architecture

- Circular dependencies between packages.
- Layer violations (e.g., CLI package importing internal runtime details).
- Leaky abstractions exposing implementation details.
- Tight coupling that prevents independent testing.
- Inconsistent patterns within the same codebase area.

### 9. Operations

- Missing or insufficient structured logging (`slog`).
- Missing error context for production debugging.
- Configuration values hardcoded instead of parameterized.
- Missing graceful shutdown handling for long-running processes.
- Observability gaps (no metrics or tracing on critical operations).

## Review Approach

- Read the PRD and TechSpec before reviewing code to understand intent.
- Review in severity order: critical first, low last.
- Focus on issues that matter. Skip style issues already caught by linters.
- Provide actionable suggestions: state the problem and what the fix looks like.
- Assign severity based on actual impact, not theoretical concern.
- Create one issue per file per distinct problem.
- If one problem spans multiple files, create one issue per affected file.
- Acknowledge well-implemented patterns; do not create issues for them.

## Work Package review contract

For target `<initiative>/WP-NNN`, compare implementation against this scope:

- Shared root context: `_prd.md`, `_techspec.md`, `_user_stories.md`,
  `_tests.md`, ADRs, and root `_work_packages.md`.
- Selected mutable context: the selected manifest-declared package directory's
  `_tasks.md`, task files, memory, review rounds, and run/diff boundary. New
  plans normally use `_packages/NNN-<brief>/`; legacy `_packages/WP-NNN/`
  remains valid.
- Excluded owned context: sibling package tasks, memory, review issues, and
  execution artifacts. Do not let identical local names such as `task_01.md` or
  `reviews-001/` collapse package identity.

Check selected package outcome, owned scope, dependency rationale, current
canonical specs, and package task manifest. If branch diff contains changes
attributable only to a sibling, report a sibling-scope warning naming paths and
why they do not belong to the selected package. Preserve the user's work: never
reset, discard, silently ignore, or rewrite those changes as part of review.

Ordinary workflow rule: absent `_work_packages.md` means existing review
discovery and scope apply unchanged. Present empty or malformed marker means
invalid opt-in and must not fall back to ordinary review.

### Clean-review completion gate

Package lifecycle completion requires all gates in order:

1. Current review creates no new issue.
2. Every earlier selected-package issue file has `status: resolved`; sibling
   issue history does not satisfy this gate.
3. Full `cy-final-verify` verification succeeds, including `make verify` where
   repository defines it.
4. Invoke hidden `compozy internal work-packages complete
   <initiative>/WP-NNN --verification-passed` bridge. The `--verification-passed`
   flag asserts gate 3 (`cy-final-verify`) succeeded; the bridge defaults it to
   `false` and refuses completion with `verification_failed` when omitted, so
   never drop it. Bridge owns locking, latest-plan reread, stable-ID checkbox
   mutation, atomic write, post-write validation, and sync.

Never manually rewrite `[ ]` to `[x]` in review guidance. Bridge is hidden
integration primitive, not additional user lifecycle step. It must not create or
switch branches, commit, push, open/merge PRs, inspect remote Git state, or
advance to another package.

Always report these independent fields:

```text
review_clean=true|false
completion_recorded=true|false
sync_pending=true|false
```

Clean review can coexist with `completion_recorded=false` when plan is missing,
malformed, read-only, or completion conflicts; preserve review evidence and name
the blocking plan outcome. Repeating completion for an already checked stable ID
is idempotent success and must not duplicate a node or rewrite unrelated bytes.
