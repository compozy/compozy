# Issue 009

- Status: RESOLVED
- Disposition: VALID
- File: `internal/cli/extension/display_test.go`
- Review request: add `t.Parallel()` to independent setup-hint tests.
- Validation: the new setup-hint tests were independent and did not currently opt into parallel execution.
- Action taken: added `t.Parallel()` to the install and enable setup-hint tests.
