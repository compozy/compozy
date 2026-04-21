---
status: resolved
file: internal/cli/daemon.go
line: 355
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58j9Bq,comment:PRRC_kwDORy7nkc655wbs
---

# Issue 004: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Errors from `os.Setenv`/`os.Unsetenv` are silently ignored.**

Per coding guidelines, errors should not be ignored with `_`. While these rarely fail, silent failures could cause hard-to-debug issues in edge cases (e.g., hitting env var limits). At minimum, log a warning.


<details>
<summary>🛡️ Suggested fix</summary>

```diff
 func overrideDaemonWebDevProxyEnv(value string) func() {
 	previousValue, hadPrevious := os.LookupEnv(daemonWebDevProxyEnv)
 	if strings.TrimSpace(value) == "" {
 		return func() {}
 	}
-	_ = os.Setenv(daemonWebDevProxyEnv, value)
+	if err := os.Setenv(daemonWebDevProxyEnv, value); err != nil {
+		// Best-effort override; log but don't fail startup
+		slog.Warn("failed to override env", "env", daemonWebDevProxyEnv, "error", err)
+	}
 	return func() {
 		if hadPrevious {
-			_ = os.Setenv(daemonWebDevProxyEnv, previousValue)
+			_ = os.Setenv(daemonWebDevProxyEnv, previousValue) // best-effort restore
 			return
 		}
-		_ = os.Unsetenv(daemonWebDevProxyEnv)
+		_ = os.Unsetenv(daemonWebDevProxyEnv) // best-effort cleanup
 	}
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/daemon.go` around lines 342 - 355, The helper
overrideDaemonWebDevProxyEnv currently ignores errors from
os.Setenv/os.Unsetenv; update it to check returned errors and log a warning when
any Setenv/Unsetenv fails rather than assigning to `_`. Specifically, in
overrideDaemonWebDevProxyEnv (and in the returned restoration closure) replace
the ignored errors for os.Setenv(daemonWebDevProxyEnv, value),
os.Setenv(daemonWebDevProxyEnv, previousValue) and
os.Unsetenv(daemonWebDevProxyEnv) with error checks and call the project's
logger (or fmt.Fprintf(os.Stderr,...)) to emit a warning including the
daemonWebDevProxyEnv name and the error; keep the existing control flow (respect
hadPrevious) but ensure all env syscall failures are surfaced via warnings.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:9ee1e28c-a6fc-4545-b8a4-38a95e0f060b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
- `overrideDaemonWebDevProxyEnv()` currently discards the return values from both `os.Setenv` and `os.Unsetenv`.
- Root cause: the helper treats process-environment mutation as infallible, so bootstrap-time override failures and cleanup failures are both silent.
- Fix plan: return the initial `Setenv` failure to the caller, keep cleanup best-effort by returning restore errors from the closure, and log restore failures at the call site.
- Minimal extra test scope is required in `internal/cli/daemon_commands_test.go` because the helper lives in `internal/cli/daemon.go` and needs direct regression coverage for successful restoration and rejected invalid values.
- Implemented: `overrideDaemonWebDevProxyEnv` now returns initial `os.Setenv` failures, returns restore errors from the cleanup closure, and the caller logs restore failures with `slog.Warn`.
- Verification:
- `go test ./internal/cli -run 'Test(CLIDaemonRunOptionsFromEnvRejectsInvalidWebDevProxyTarget|ResolveDaemonWebDevProxyTargetRejectsInvalidFlagValueWithFlagContext|OverrideDaemonWebDevProxyEnv|DaemonStartCommandFlagOverridesInvalidWebDevProxyEnv|DaemonStartCommandForegroundUsesDaemonRunner)$' -count=1`
- `make verify`
