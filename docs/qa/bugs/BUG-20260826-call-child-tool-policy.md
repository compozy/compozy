# BUG-20260826-call-child-tool-policy: Unrestricted call children could not return results

- **Status:** fixed, pending public-surface retest
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno; Ada
- **Journey Step:** J-delegate-work-to-an-agent — complete a typed call through the child return tool
- **Scenarios:** RT-agent-call-golden-path; RT-call-return-contract-repair
- **Found:** 2026-08-26 · **Report:** `docs/qa/reports/2026-08-26-agent-comms.md`

## Summary

A call child created from an unrestricted logical caller received an empty concrete tool policy.
The hosted tool catalog therefore marked `compozy__call_return` as denied for the child, so the
agent could finish its work but could not settle the durable call.

## Reproduction

1. Create an operator-originated call targeting an unrestricted workspace agent.
2. Wait for the child session to finish its prompt.
3. Inspect the child-scoped tool catalog with `compozy tool list`.
4. Observe the durable call state.

**Expected:** The child can invoke `compozy__call_return`, and the durable call settles.
**Actual:** The catalog reported `session_denied` for `compozy__call_return`; the child became idle
while the call remained `running`.

## Fix

- **Root cause:** An unrestricted root policy was materialized as no concrete tools, and omitted call
  narrowing categories were treated as empty instead of inheriting the caller policy.
- **Production fix:** Logical root sessions now materialize the complete native tool universe before
  applying deny patterns. Omitted call permission categories inherit the normalized caller policy;
  explicit categories remain narrowed.
- **Regression:** The canonical call admission suite proves omitted categories inherit from the
  caller, and the canonical session lineage suite proves an unrestricted logical root persists the
  native tool universe.
- **Fix commit:** pending QA remediation commit.

## Verification

- Focused regressions: `go test -race ./internal/calls ./internal/session -run 'TestServiceCreateAdmissionAndActivation|TestCreateAllowedToolsOverrideNarrowsAgentProfile' -count=1` — 16 tests passed.
- Package suites: `go test -race ./internal/calls ./internal/session -count=1` — passed.
- Public child catalog and settlement retest is pending a rebuilt isolated daemon.
