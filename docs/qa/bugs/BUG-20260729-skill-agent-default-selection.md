# BUG-20260729-skill-agent-default-selection: Agent skill settings opened with an unavailable identity

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-agent-marketplace-parity, manage one agent's skill tombstones
- **Scenarios:** ET-012
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated browser replay

## Summary

Entering Agent scope on the Skills settings page selected the first alphabetically sorted fleet
agent, `code_implementer`. That extension-projected identity is not the canonical authored default
accepted by the agent-scoped settings read, so the entire settings page was replaced by a 404 error.

## Reproduction

1. Start the isolated daemon with the bundled `spec-cycle` extension active.
2. Open `/settings/skills` and select **Agent** scope.
3. Observe the selected agent and the resulting page state.

**Expected:** Agent scope opens on the runtime's canonical `general` agent and keeps the settings
surface usable.
**Actual before the fix:** The page selected `code_implementer` and rendered
`settings: agent "code_implementer" not found` in place of the settings surface.

## Evidence

- Browser before/after and adjacent lifecycle replay:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/014-skills-browser`.
- Deterministic 1440×900 route capture:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/014-skills-browser/deterministic/settings-skills.png`.
- The canonical hook regression failed before the production change with
  `agentName: "code_implementer"`, then passed with `agentName: "general"`.

## Fix

- **Root cause:** The settings view-model treated lexical fleet order as a default-agent policy.
  Extension-projected agents sort before the runtime's authored default, even though
  `compozyconfig.DefaultAgentName` and bootstrap authoring guarantee `general` as the default.
- **Correction:** The view-model now selects `general` when present and retains the prior
  first-item fallback only for deployments that intentionally omit the canonical default.
- **Fix commit:** pending completion gate
- **Regression test:** `Should select the canonical general agent when entering agent scope` in
  `web/src/systems/settings/hooks/__tests__/use-settings-skills-page.test.tsx`.

## Verification

- The rebuilt local SPA selected `general`, retained the full settings page, and applied an
  agent-local `compozy` tombstone removal immediately; the daemon then reported an empty tombstone
  list and `compozy.enabled=true`.
- `bunx turbo run lint typecheck test --filter=./web` passed: 515 files and 4,047 tests.
- **Retested:** pending fix commit
