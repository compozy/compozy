# Issue 016

- Status: RESOLVED
- Disposition: VALID
- File: `internal/setup/legacy_cleanup.go`
- Review request: keep legacy cleanup running when an agent does not support the requested scope.
- Validation: any scope-unsupported path resolution would previously abort the entire cleanup run.
- Action taken: introduced a sentinel unsupported-scope error from `resolveInstallPaths`, skipped those combinations during legacy cleanup, and added regression coverage.
