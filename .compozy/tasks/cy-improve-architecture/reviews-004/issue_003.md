---
provider: manual
pr: 6
round: 4
round_created_at: 2026-07-14T02:01:21Z
status: resolved
file: extensions/cy-improve-architecture/evals/skill_e2e_test.go
line: 281
severity: medium
author: claude-code
provider_ref:
---

# Issue 003: E2E installs skills only for Codex

## Review Comment

`executeAudit` selects its runtime from `COMPOZY_E2E_IDE`, but
`installShippedSkills` always installs the extension pack with
`AgentNames: []string{"codex"}`. When the opt-in evaluator runs with Claude,
Droid, or another supported IDE, that runtime uses its own project skill
location and therefore cannot see the installed audit skill. The evaluation can
then fail for the wrong reason or run an unskilled prompt instead of testing the
extension.

Install the pack for the selected evaluation runtime (for example, pass
`evaluationIDE()` into `AgentNames`) and cover at least one non-Codex runtime
mapping. Keep Codex as the default when no environment override is supplied.

Source: PR #6, unresolved review thread at line 281.

## Triage

- Decision: `VALID`
- Root cause: `executeAudit` honors `COMPOZY_E2E_IDE`, while
  `installShippedSkills` hard-codes the unrelated setup agent name `codex`.
  An evaluation run with a different supported runtime therefore installs the
  pack in Codex's project directory rather than the directory that runtime
  reads.
- Fix approach: map the selected runtime with `setup.AgentNameForIDE` before
  invoking the extension-pack installer, preserve Codex as the default through
  `evaluationIDE`, and add a unit regression test for the default and the
  `claude` to `claude-code` mapping. The existing opt-in ACP evaluation
  exercises the real installer and runtime boundary.

## Resolution

- `installShippedSkills` now resolves the selected evaluation IDE to its setup
  agent before installation, so the skill pack is materialized in that
  runtime's project skill directory.
- The regression test invokes the real local extension-pack installer for the
  default runtime and for Claude, asserting the audit skill exists only in the
  selected runtime's directory.
- Verified with `go test ./extensions/cy-improve-architecture/evals -count=1`
  and the repository's full `make verify` gate.
