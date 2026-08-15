# BUG-20260802-extension-agent-edit-reset: Extension agent edits were overwritten during reconcile

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, configure and run an enabled kit agent
- **Scenarios:** ET-ext-kit-enable; ET-020; ET-021; ET-022
- **Found:** 2026-08-02 · **Report:** docs/qa/reports/2026-08-02-bundles-removal.md
- **Origin:** Task 06 real-user QA

## Summary

An operator could update an extension-owned agent through the public compare-and-swap API and
receive a successful response, but the next extension reconcile restored the install-time agent
snapshot. The accepted edit therefore disappeared, and extension-owned Loops could not retain the
provider and command selected for their agents.

## Reproduction

1. Enable the bundled `spec-cycle` extension.
2. Update one of its published agents through `PUT /api/agents/:name` with the current definition
   digest and a deterministic ACP fixture command.
3. Trigger extension resource reconcile, then read or execute the agent again.

**Expected:** Reconcile reloads the current extension agent files, so a public agent edit that was
persisted to those files remains authoritative.
**Actual:** Reconcile republishes the cached install-time `StaticAgents` value and overwrites the
accepted edit.

## Evidence

- Initial daemon E2E failures: `/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa/test-e2e-runtime.log`.
- Green full rerun: `/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa/test-e2e-runtime-rerun.log`.

## Fix

- **Root cause:** `extensionAgentSkillDeclarationProvider` reconciled agents from the cached
  `Extension.StaticAgents` snapshot instead of reloading the enabled extension's current
  dir-per-agent files.
- **Correction:** Agent publication now reloads the manifest-declared agent directories before each
  reconcile. The affected E2E fixtures explicitly enable the extension and configure its own agents
  through the public compare-and-swap API.
- **Fix commit:** `4f1ceef`
- **Regression test:** `internal/daemon/daemon_extension_agent_fixture_e2e_integration_test.go`
  owns the public extension-agent update helper; the agent-definition, extension-command,
  review-and-fix, and software-delivery daemon E2E suites exercise the repaired lifecycle.

## Verification

- The four focused daemon E2E scenarios passed under `-race` after the production fix.
- The complete runtime E2E lane passed: daemon 133, HTTP 8, UDS 32, and testutil/e2e 8.

