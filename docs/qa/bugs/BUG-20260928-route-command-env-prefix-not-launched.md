# BUG-20260928-route-command-env-prefix-not-launched: A fallback route's account command never launches

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Rafa, Ada
- **Journey Step:** J-route-background-work (seat/role fallback to an account route)
- **Scenarios:** RT-session-fallback-chain; MS-background-role-fallback
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b1.md

## Summary

The documented account form of a route command — `CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp`, i.e. leading
`NAME=value` assignments — is accepted by `agent create --fallback-route` / `[roles.*.fallback_chain]` (config validation parses
it with `ParseLaunchCommand`), but the ACP launcher split the command naively and tried to execute `NAME=value` as the program.
Seat two was then "refused before acceptance" too and the chain exhausted: `executable file not found in $PATH: QA_ACCOUNT=seat-two`.

## Reproduction

`compozy agent create seat-reviewer --provider acpmock-seat --model fallback-title-model --fallback-route "provider=acpmock-seat,model=fallback-title-model,command=QA_ACCOUNT=seat-two <acpmock-driver …>"`,
then `session new --agent seat-reviewer` + a prompt → `fallback chain exhausted after 2 attempt(s) … executable file not found in $PATH: QA_ACCOUNT=seat-two`.
The same command on `roles.auto_title.fallback_chain` fails identically. Integration coverage used plain commands only.

## Root cause

`localLauncher.PrepareLaunch` (`internal/acp/launch_identity.go`) used `parseCommandString` (plain shell split) instead of the
single launch grammar `compozyconfig.ParseLaunchCommand`, which separates leading assignments as private environment.

## Fix

`PrepareLaunch` parses with `ParseLaunchCommand` and sets each leading assignment on the child environment (overriding the
inherited value); argv starts at the executable. Regression: `TestLocalLauncherPrepareLaunchForwardsLeadingEnvironment`
(`internal/acp/launcher_tool_host_test.go`).

## Verification

Re-walk: `seat-reviewer` prompt streams on seat two (`docs/qa/evidence/2026-09-28-session-continue-fork-b1/seat-p1.txt`), one `session.fallback.used` with the command
fingerprint and no command text (`docs/qa/evidence/2026-09-28-session-continue-fork-b1/seat-fallback-logs.json`); the auto-title role titles the session through the
env-prefixed route with one `role.fallback.used` (`docs/qa/evidence/2026-09-28-session-continue-fork-b1/role-fallback-logs.json`).
