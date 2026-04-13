# Issue 012

- Status: RESOLVED
- Disposition: VALID
- File: `internal/core/extension/origin_test.go`
- Review request: add the symmetric empty-directory test for `LoadInstallOrigin`.
- Validation: only the write path had empty-directory coverage.
- Action taken: added `TestLoadInstallOriginRejectsEmptyDirectory`.
