# BUG-20260827-default-profile-agent-prompt-resolution: Default-profile agents fail before their first prompt

- **Status:** fixed, pending remediation commit
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada
- **Journey Step:** J-18 Start a session from the authored agent and send its first prompt
- **Scenarios:** RT-070
- **Found:** 2026-08-27 · **Report:** docs/qa/reports/2026-08-27-acp-runtime-catalog.md

## Summary

A workspace agent created under the default profile could start a session, but its first prompt failed while Compozy assembled the available command catalog.

## Reproduction

- **Charter:** CH-agent-runtime-default-options · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US; isolated local daemon with the default profile and a workspace-scoped agent

1. Create a workspace agent under the default profile.
2. Create a session from that agent.
3. Send the first prompt.

**Expected:** Compozy resolves the agent and its skills through the active profile, then sends the prompt to the provider.
**Actual:** Prompt preparation failed with `skills: agent not found: "grok_runtime_writer"` before the provider received the turn.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/session-profile-agent-prompt-failed.png`

## Fix

- **Root cause:** the prompt-skills resolver special-cased the default profile as if no profile were active. It called the unprofiled workspace resolver and omitted `.compozy/profiles/default/agents`.
- **Fix commit:** pending; included in the single remediation commit
- **Regression test:** `TestSessionCommandServiceProjectsAndRevalidatesExactSkillSources/Should resolve default-profile commands through the profile layer`

## Verification

- **Retested:** 2026-08-27 in the same isolated lab after rebuilding and restarting the daemon
- **Result:** pass — a fresh default-profile `grok_runtime_writer` session completed command discovery and reached the Cursor provider on its first prompt.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/cursor-grok-runtime-after-fix.json`
