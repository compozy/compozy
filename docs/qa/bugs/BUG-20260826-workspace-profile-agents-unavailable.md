# BUG-20260826-workspace-profile-agents-unavailable: Workspace profile agents cannot start sessions

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** isolated workspace setup, create an agent definition and start its session
- **Scenarios:** MS-repo-profile-layer-adoption
- **Found:** 2026-08-26 · **Report:** docs/qa/reports/2026-08-26-integrated-terminal.md

## Summary

An agent created for a workspace's `default` profile appears in `agent list` and `agent info`, but the
same workspace rejects `session new --agent <name>` with `agent not available in workspace`. Restarting
the daemon and passing `--profile default` do not change the result.

## Reproduction

- **Charter:** QA bootstrap prerequisite · **Tour:** Feature Tour
- **Environment:** isolated macOS runtime / CLI / wifi-fast / en-US

1. Register a workspace.
2. Run `agent create <name> --workspace <workspace-id> --profile default`.
3. Confirm the definition exists under `.compozy/profiles/default/agents/<name>/AGENT.md` and appears
   in `agent list` and `agent info` for that workspace.
4. Run `session new --workspace <workspace-id> --agent <name> --profile default`.

**Expected:** Session resolution uses the same workspace-profile agent catalog exposed by agent reads.
**Actual:** Session creation returns `agent not available in workspace`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/agent-create.jsonl`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/workspace-agent-snapshot-global-fallback.json`
- The real-scenario run proceeded only after creating identical definitions in the isolated global
  profile, proving the provider and agent definitions themselves were valid.

## Fix

- **Root cause:** session startup resolved agents from the global snapshot instead of the selected
  workspace-profile catalog.
- **Fix commit:** current branch
- **Regression test:** `internal/session/manager_test.go` in the existing workspace-profile suite.

## Verification

- **Retested:** the focused manager regression and isolated real-scenario activation pass.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/operator-kickoff.jsonl` and `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/task-activation.json`.
