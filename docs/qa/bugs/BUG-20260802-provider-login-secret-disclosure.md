# BUG-20260802-provider-login-secret-disclosure: Provider reads reveal a login environment secret

- **Status:** verified
- **Impact (user-side):** Data-Loss
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Dora
- **Journey Step:** J-administer-provider-auth, inspect the safe login descriptor after configuration
- **Scenarios:** RT-025; RT-026; RT-027
- **Found:** 2026-08-02 · **Report:** docs/qa/reports/2026-08-02-provider-auth-hard-cut.md
- **Origin:**

## Summary

A runtime administrator can write a login command that starts with an environment assignment, but provider inventory, provider detail, and auth-probe reads return that assignment as the supposedly safe executable basename. A value intended to remain private is therefore disclosed through ordinary CLI and HTTP reads.

## Reproduction

- **Charter:** CH-provider-auth-surfaces-dora · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US; fresh isolated daemon at `127.0.0.1:40279`

1. Set `providers.qa-native.auth_login_command` through `compozy config set` to a command with an environment prefix, an absolute executable, and arguments.
2. Confirm the mutation response and `compozy config get` return `[redacted]`.
3. Read `compozy provider list`, `GET /api/providers/qa-native`, or `POST /api/providers/qa-native/auth/probe`.
4. Observe that `login.executable` contains the environment assignment instead of the executable basename.

**Expected:** Every read returns only the basename of the executable and never returns the environment prefix, arguments, or resolved path.
**Actual:** CLI and HTTP reads expose the environment assignment in `login.executable`.

## Evidence

- `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/evidence/provider-auth/config-set-login-redacted.json`
- `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/evidence/provider-auth/config-get-login-redacted.json`
- `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/evidence/provider-auth/http-provider-login-prefix-leak.json`
- `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/evidence/provider-auth/http-probe-before-login.json`

## Fix

- **Root cause:** Provider login parsing treated the first shell token as the executable everywhere. For commands beginning with `NAME=value`, that token was therefore projected as the executable and passed through executable resolution instead of remaining private process environment.
- **Fix commit:** working tree; this QA remediation is not committed separately.
- **Regression test:** `internal/providers/classify_test.go` proves the safe descriptor uses the real executable basename and excludes environment names/values, arguments, paths, and secrets; `internal/providers/runner_test.go` proves leading assignments are applied to the child environment while resolution and argv use the real executable; `internal/cli/provider_test.go` proves login output excludes every private command component.

## Verification

- **Retested:** 2026-08-03 in the same fresh Dora charter after rebuilding and restarting the isolated daemon twice.
- **Result:** verified fixed. CLI/UDS inventory, HTTP detail/probe, Settings, Doctor, Web, CLI config, `compozy__config_set`, internal CLI login, and post-restart reads expose only `qa-login-runner`. The login environment and arguments still executed successfully, as proved by the controlled fixture reaching `authenticated`. Focused regressions passed 20 times under `-race`; the complete affected provider/CLI/API/Settings suites, vet, zero-warning lint, Windows build, convention checks, formatting, and diff checks pass.
- **Evidence:** `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/evidence/provider-auth/retest-cli-provider-list-qa-native.json`; `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/evidence/provider-auth/retest-http-provider-safe.json`; `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/evidence/provider-auth/retest-native-config-set-redacted.json`; `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/evidence/provider-auth/retest-post-restart-authenticated.json`; `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/evidence/provider-auth/web-provider-safe-login-cli.png`.
