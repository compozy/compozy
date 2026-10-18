# BUG-20260917-clarify-timeout-config-set: `config set tools.clarify.timeout` denied though the spec names it as the operating surface

- **Status:** verified — allowlist admits the duration kind, structured writer re-walk passed
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, policy write step
- **Scenarios:** MS-clarify-timeout-policy; RT-session-clarification-roundtrip
- **Found:** 2026-09-17 · **Report:** docs/qa/reports/2026-09-17-clarify-keepalive.md
- **Origin:** clarify-keepalive task_05 live walk (CH-clarify-policy-matrix, Garbage Tour)

## Summary

Dora follows the documented flow to bound clarification waits — `compozy config set tools.clarify.timeout 5m --scope user -o json`, the exact transcript in the spec's own agent contract — and the CLI refuses with `cli: config path "tools.clarify.timeout" is not supported by config set`. The key could only be changed by hand-editing `config.toml`, contradicting the `_dx.md`/`_spec.md` contract and the MS scenario entry points.

## Reproduction

- **Charter:** CH-clarify-policy-matrix · **Tour:** Garbage Tour
- **Environment:** desktop / wifi-fast / en-US, isolated QA lab (COMPOZY_HOME=/tmp/compozyqa-88565633a518/runtime, daemon on 127.0.0.1:39383)

1. Boot a clean daemon (omitted key) and confirm `config get tools.clarify.timeout -o json` → `"0s"`.
2. Run `compozy config set tools.clarify.timeout 5m --scope user -o json`.

**Expected:** the `_dx.md` transcript — structured write recording desired state with `"lifecycle": "restart-required"`, `"applied": false`, `"next_action": "restart-daemon"`.
**Actual:** `error: cli: config path "tools.clarify.timeout" is not supported by config set`.

## Evidence

- Lab log `qa-artifacts/qa/logs/policy-P2-5m.log` (pre-fix denial captured at 2026-09-17T23:34Z) vs `qa-artifacts/qa/logs/policy-Q1-set-walk.log` (post-fix acceptance).
- Dedup: `grep -rln clarify docs/qa/bugs/` matched only BUG-20260803-extension-session-grant-denied (extension install flow) and BUG-20260906-native-profile-agent (registry policy resolution) — different symptoms, no duplicate.

## Fix

- **Root cause:** tasks 01–03 admitted and validated the `tools.clarify.timeout` policy in `internal/config/tools.go` but never registered the path in the agent-mutable allowlist (`agentMutableConfigKinds` in `internal/config/tool_surface.go`), so `classifyConfigMutationPath` fell through to the "not supported by config set" denial. Spec/contract edits and code edits were owned by different tasks and the seam was never walked live until task_05.
- **Fix commit:** uncommitted worktree change (this task leaves changes uncommitted per delivery policy): one constant `toolSurfaceToolsClarifyTimeoutPath = "tools.clarify.timeout"` plus one `ConfigValueDuration` map entry in `internal/config/tool_surface.go`; one allow-case in `internal/config/tool_surface_test.go` pinning the contract.
- **Regression test:** new `Should allow clarify timeout mutation` case in `TestToolConfigPathPolicy*` (`internal/config/tool_surface_test.go`) — denied before (fall-through denial), allowed after — plus the full `internal/cli` + `internal/config` suites and the live Q1–Q4 re-walk (set 5m/0s accepted with restart-required lifecycle; set 99h rejected with the exact policy error and last-valid-kept; unset restores omitted).

## Verification

- **Retested:** 2026-09-17, Dora/J-administer-runtime-settings + Théo/J-answer-agent-requests (E2E-001/E2E-002 unaffected, re-ran green) · **Report:** docs/qa/reports/2026-09-17-clarify-keepalive.md
- **Result:** `config set`/`unset`/`get`/`validate`/`reload` agree across omitted/`0s`/finite/invalid shapes; spec-owned lanes green (`make go-lint`, `go test ./internal/cli/ ./internal/config/`, `-race` clarify suites); `make gate` failed only at the pre-existing Daytona `codegen-check` lane.
