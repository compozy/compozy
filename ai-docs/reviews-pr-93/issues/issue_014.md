# Issue 014

- Status: RESOLVED
- Disposition: VALID
- File: `internal/setup/catalog_effective_test.go`
- Review request: mark `reusableAgentNames` as a test helper.
- Validation: the helper did not call `t.Helper()`.
- Action taken: changed the signature to accept `*testing.T`, called `t.Helper()`, and updated the caller.
