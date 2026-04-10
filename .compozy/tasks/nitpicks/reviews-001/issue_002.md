---
status: resolved
file: internal/cli/workspace_config_test.go
line: 152
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4090534249,nitpick_hash:5359ddd0ae2f
review_hash: 5359ddd0ae2f
source_review_id: "4090534249"
source_review_submitted_at: "2026-04-10T14:58:54Z"
---

# Issue 002: Use a single table-driven t.Run("Should...") block for these nitpicks cases.
## Review Comment

These two tests cover the same setup with one override toggle. Folding them into a small table keeps the fetch-reviews workspace-default coverage aligned and avoids adding more one-off top-level cases here.

As per coding guidelines, "`**/*_test.go`: Use table-driven tests with subtests (`t.Run`) as the default pattern for Go tests`" and "`MUST use t.Run(\"Should...\") pattern for ALL test cases`".

## Triage

- Decision: `valid`
- Root cause: The two fetch-reviews nitpicks workspace-default tests duplicate nearly identical setup and differ only in whether the command flag is explicitly set, so they drift away from the repo's default table-driven/subtest pattern.
- Impact: Leaving them as separate one-off top-level tests makes this area harder to extend and inconsistent with the Go testing conventions enforced elsewhere in the repository.
- Fix approach: Collapse the two cases into a single table-driven test with `t.Run("Should ...")` subtests so the shared setup stays centralized and each behavior branch remains explicit.
- Resolution: The workspace-default nitpicks coverage now lives in one table-driven test with `t.Run("Should ...")` subtests for the config-default and explicit-flag cases.
- Verification: `go test ./internal/cli ./internal/core ./internal/core/provider/coderabbit` passed, and `env -u COMPOZY_NO_UPDATE_NOTIFIER make verify` passed cleanly.
