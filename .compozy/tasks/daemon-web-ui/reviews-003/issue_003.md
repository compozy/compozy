---
status: resolved
file: internal/cli/daemon.go
line: 320
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149317620,nitpick_hash:9e051c02296b
review_hash: 9e051c02296b
source_review_id: "4149317620"
source_review_submitted_at: "2026-04-21T16:29:30Z"
---

# Issue 003: Error messages misleadingly reference env var name when validating flag values.
## Review Comment

When `normalizeDaemonWebDevProxyTarget` is called from `resolveDaemonWebDevProxyTarget` with a flag value, error messages like `%s=%q must use http or https` still use `daemonWebDevProxyEnv` constant, confusing users who passed `--web-dev-proxy ws://...`.

Consider passing a source label parameter or restructuring to provide accurate error context.

## Triage

- Decision: `valid`
- Notes:
- `resolveDaemonWebDevProxyTarget()` passes flag values straight into `normalizeDaemonWebDevProxyTarget()`, but the normalizer always formats validation errors with `daemonWebDevProxyEnv`.
- Root cause: the validation helper has no source-context parameter, so flag-originated failures are mislabeled as environment-variable failures.
- Fix plan: thread an explicit source label through the normalizer so `--web-dev-proxy` errors reference the flag while env-driven errors still reference `COMPOZY_WEB_DEV_PROXY`.
- Minimal extra test scope is required in `internal/cli/daemon_commands_test.go` because `internal/cli/daemon.go` does not currently have focused unit coverage for the helper error text.
- Implemented: added `daemonWebDevProxyFlag`, threaded a source label through `normalizeDaemonWebDevProxyTarget`, and preserved env-context errors for `COMPOZY_WEB_DEV_PROXY`.
- Verification:
- `go test ./internal/cli -run 'Test(CLIDaemonRunOptionsFromEnvRejectsInvalidWebDevProxyTarget|ResolveDaemonWebDevProxyTargetRejectsInvalidFlagValueWithFlagContext|OverrideDaemonWebDevProxyEnv|DaemonStartCommandFlagOverridesInvalidWebDevProxyEnv|DaemonStartCommandForegroundUsesDaemonRunner)$' -count=1`
- `make verify`
