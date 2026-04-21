---
status: resolved
file: internal/cli/daemon.go
line: 167
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58jarr,comment:PRRC_kwDORy7nkc655CwU
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**`--web-dev-proxy` cannot override a bad `COMPOZY_WEB_DEV_PROXY`.**

`cliDaemonRunOptionsFromEnv()` validates the env var before the flag override runs, so `COMPOZY_WEB_DEV_PROXY=ws://... compozy daemon start --web-dev-proxy http://127.0.0.1:3000` still fails even though the explicit flag should win. Resolve the proxy target with flag-or-env precedence first, then build `daemon.RunOptions` from that result.


<details>
<summary>💡 Suggested direction</summary>

```diff
-	runOptions, err := cliDaemonRunOptionsFromEnv()
+	httpPort, err := cliDaemonHTTPPortFromEnv()
 	if err != nil {
-		return err
+		return withExitCode(1, err)
 	}
+	webDevProxyTarget := ""
 	if strings.TrimSpace(s.webDevProxyTarget) != "" {
-		runOptions.WebDevProxyTarget, err = normalizeDaemonWebDevProxyTarget(s.webDevProxyTarget)
+		webDevProxyTarget, err = normalizeDaemonWebDevProxyTarget(s.webDevProxyTarget)
 		if err != nil {
 			return withExitCode(1, err)
 		}
+	} else {
+		webDevProxyTarget, err = cliDaemonWebDevProxyFromEnv()
+		if err != nil {
+			return withExitCode(1, err)
+		}
 	}
+	runOptions := daemon.RunOptions{
+		Version:           version.String(),
+		HTTPPort:          httpPort,
+		WebDevProxyTarget: webDevProxyTarget,
+	}
```
</details>


Also applies to: 265-278

<!-- fingerprinting:phantom:medusa:grasshopper:2fe85954-b1fb-4290-8e9e-00be8dcc48f0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `daemonStartState.run()` currently calls `cliDaemonRunOptionsFromEnv()` before applying the `--web-dev-proxy` flag, so an invalid `COMPOZY_WEB_DEV_PROXY` fails the command even when the flag provides a valid explicit override.
  - That violates expected flag-over-env precedence for operator commands.
  - Implemented: daemon start now resolves the HTTP port and web dev proxy target separately, applying the explicit flag before consulting the env var; added `TestDaemonStartCommandFlagOverridesInvalidWebDevProxyEnv` in `internal/cli/daemon_commands_test.go` because there was no in-scope CLI precedence test harness.
  - Verification: `go test ./internal/cli -run 'Test(DaemonStartCommandFlagOverridesInvalidWebDevProxyEnv|CLIDaemonRunOptionsFromEnvRejectsInvalidWebDevProxyTarget)$' -count=1`
  - Repo gate note: the attempted full `make verify` run stopped earlier in `frontend:bootstrap` because of the unrelated pre-existing `package.json`/`bun.lock` mismatch.
