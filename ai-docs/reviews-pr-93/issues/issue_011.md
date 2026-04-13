# Issue 011

- Status: RESOLVED
- Disposition: VALID
- File: `internal/cli/extension/install.go`
- Review request: remove the unnecessary nil fallback around `deps.writeInstallOrigin`.
- Validation: `TestDefaultCommandDepsAndHelperFunctions` guarantees the dependency is non-nil.
- Action taken: removed the defensive fallback and now call `deps.writeInstallOrigin` directly.
