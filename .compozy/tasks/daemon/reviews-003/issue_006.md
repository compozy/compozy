---
status: resolved
file: internal/cli/root_command_execution_test.go
line: 1641
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4135415269,nitpick_hash:c2468da10cbf
review_hash: c2468da10cbf
source_review_id: "4135415269"
source_review_submitted_at: "2026-04-19T03:14:21Z"
---

# Issue 006: Assert the request built from the interactive form.
## Review Comment

These tests currently prove only that the stub returned its canned response / run ID. They would still pass if `name`, `provider`, `pr`, or `round` from `collectForm` were never propagated into the daemon request. Please assert the captured request fields too. As per coding guidelines, "Ensure tests verify behavior outcomes, not just function calls".

Also applies to: 1657-1662, 1730-1735, 1745-1750

## Triage

- Decision: `valid`
- Root cause: `TestReviewsFetchCommandNoFlagsUsesInteractiveForm` and `TestReviewsFixCommandNoFlagsUsesInteractiveForm` only assert the stubbed fetch summary or run ID. They never verify that the values populated by `collectForm` are forwarded into the daemon `FetchReview` or `StartReviewRun` requests.
- Why this is valid: without asserting the captured request fields, these tests would still pass if the CLI stopped propagating the interactive `name`, `provider`, `pr`, or `round` values into the daemon transport layer, so the tests do not fully protect the behavior they are supposed to cover.
- Fix approach: switch the tests to a request-capturing daemon client and assert the resolved workspace, workflow slug, and the fetch/run request fields built from the interactive form before closing the issue.
- Resolution: updated the interactive no-flags review tests in `internal/cli/root_command_execution_test.go` to use `reviewExecCaptureClient` and assert the daemon fetch/review-run workspace, workflow slug, provider, PR, round, and interactive presentation mode generated from `collectForm`.
- Regression coverage: the fetch test now fails if interactive form values stop populating `ReviewFetchRequest`, and the fix test now fails if the form-derived workflow slug or round stop reaching `StartReviewRun`. While strengthening that coverage, the fetch fixture was also corrected to populate `state.round = 1`, which the new assertions immediately exposed as missing test setup.
- Verification: `go test ./internal/cli -run 'TestReviews(Fetch|Fix)CommandNoFlagsUsesInteractiveForm$' -count=1` passed. A fresh `make verify` also passed end to end, including formatting, zero lint issues, `2404` tests with `1` skipped helper-process case, and a successful `go build ./cmd/compozy`.
