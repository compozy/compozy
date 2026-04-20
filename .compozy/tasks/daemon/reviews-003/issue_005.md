---
status: resolved
file: internal/cli/root_command_execution_test.go
line: 1488
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4135415269,nitpick_hash:d6e960a73d1b
review_hash: d6e960a73d1b
source_review_id: "4135415269"
source_review_submitted_at: "2026-04-19T03:14:21Z"
---

# Issue 005: Use a table-driven test for the removed-command coverage.
## Review Comment

These three cases are the same test with a different command string. Folding them into a single `t.Run("Should...")` table would remove duplication and make future command removals cheaper to cover. As per coding guidelines, "Use table-driven tests with subtests (`t.Run`) as the default pattern" and "Check for shared test utilities usage to avoid duplication".

---

## Triage

- Decision: `valid`
- Root cause: `internal/cli/root_command_execution_test.go` covers removed legacy commands with four standalone tests that all execute the same assertion flow and only vary by the command name and expected unknown-command message.
- Why this is valid: the repository guidance prefers table-driven tests with subtests by default, and this duplication makes future command removals more expensive to maintain because each added legacy command requires another copy of the same test body.
- Fix approach: replace the duplicated removed-command tests with one table-driven test that iterates over the removed command names and asserts the expected Cobra unknown-command output for each case.
- Resolution: consolidated the duplicated `start`, `validate-tasks`, `fetch-reviews`, and `fix-reviews` removal checks into `TestLegacyCommandsAreRemoved`, a single table-driven test with per-command subtests in `internal/cli/root_command_execution_test.go`.
- Regression coverage: the new table preserves the same four command probes and the same Cobra unknown-command assertion path, while making future removed-command additions a one-line test-case change instead of another standalone test body.
- Verification: `go test ./internal/cli -run '^TestLegacyCommandsAreRemoved$' -count=1` passed. A fresh `make verify` also passed end to end, including formatting, lint, `2404` tests with `1` skipped helper-process case, and a successful `go build ./cmd/compozy`.
