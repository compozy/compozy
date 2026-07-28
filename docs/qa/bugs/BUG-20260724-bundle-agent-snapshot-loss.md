# BUG-20260724-bundle-agent-snapshot-loss: Installed bundle profiles silently lose packaged agents

- **Status:** verified
- **Impact (user-side):** Functional
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-32 reserved-name bundle materialization probe
- **Scenarios:** RT-reserved-builtin-agent-names
- **Found:** 2026-07-24 · **Report:** docs/qa/reports/2026-07-24-agent-roles.md

## Summary

A locally installed extension loaded a bundle profile that packaged `agents/coordinator/AGENT.md`, but the public bundle service received an empty agent list. Preview showed no resources and activation succeeded as an empty profile, bypassing both normal agent materialization and reserved-name validation.

## Reproduction

- **Charter:** CH-reserved-builtin-name-sweep · **Tour:** Garbage Tour
- **Environment:** desktop / wifi-fast / en-US, isolated `devtool-oss-launch` lab

1. Install an extension whose bundle profile contains `[[profiles.agents]] path = "agents/coordinator"`.
2. Preview or activate the profile through the daemon.
3. Inspect the activation inventory and agent catalog.

**Expected:** The complete packaged agent reaches bundle validation, which rejects the reserved identity before any write.
**Actual:** The profile snapshot contains zero agents; an empty activation succeeds and materializes no agent.

## Evidence

- `reserved-bundle-{extension-install,catalog-before-activate,preview-current}.json` in the run's `qa-artifacts/qa` directory.
- The accidental empty activation `act_16b6f8f030a6fd9a` was immediately deactivated; `reserved-bundle-empty-activation-deactivate.json` proves cleanup.

## Fix

- **Root cause:** `cloneBundleSpecs` copied channels, jobs, triggers, and bridges but omitted `BundleProfile.Agents`. Bundle services consume the Manager snapshot rather than the mutable installed object.
- **Fix:** Extension snapshots now deep-clone every packaged agent plus Soul and Heartbeat sidecars.
- **Fix commit:** `c841d7e06428c28e4e1b4ba8c17bccb4a103eea1`
- **Regression test:** `internal/extension/manager_test.go` — `TestManagerCloneExtensionReturnsIsolatedSnapshot` proves agents survive and mutable fields do not alias the installed extension.

## Verification

- After rebuild and restart, the same fixture reached reserved-name validation before preview or activation.
- No activation or agent catalog residue remained after the valid probe.
- The extension `-race` suite passed 789 tests; repository lint passed.
