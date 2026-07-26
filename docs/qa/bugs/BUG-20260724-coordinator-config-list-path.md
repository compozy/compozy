# BUG-20260724-coordinator-config-list-path: Coordinator enabled state is published under a path operators cannot use

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-route-background-work baseline inspection, step 1
- **Scenarios:** MS-background-role-routing
- **Found:** 2026-07-24 · **Report:** docs/qa/reports/2026-07-24-agent-roles.md

## Summary

Dora listed effective configuration before changing a background role and saw the coordinator toggle as `roles.coordinator.roleconfig.enabled`. That path is not part of the public `[roles]` contract and cannot be used with `agh config set`; the supported path is `roles.coordinator.enabled`.

## Reproduction

- **Charter:** CH-background-role-routing-scopes · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US, isolated `devtool-oss-launch` lab

1. Start the isolated daemon with pristine role defaults.
2. Run `agh config list -o json`.
3. Inspect the entries beginning with `roles.coordinator`.

**Expected:** The coordinator toggle is listed as `roles.coordinator.enabled`.
**Actual:** The list contains `roles.coordinator.roleconfig.enabled` and omits the canonical enabled path.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/config-list-roles.json`
- Independent write-path check: `agh config set roles.coordinator.max_children 6` resolves the canonical coordinator branch and returns its bounded validation error, while the published `roleconfig` segment is absent from the supported registry.

## Fix

- **Root cause:** The redacted config projector treated every exported struct field as a named TOML node. Go's anonymous `RoleConfig` field is flattened by the TOML contract, but the projector lowercased its Go type name to `roleconfig` and inserted a segment that does not exist in the file or validated write registry.
- **Fix:** Anonymous untagged TOML struct fields are now merged into their parent projection while explicit outer fields retain precedence.
- **Fix commit:** `69b2099f3cada66395ced4c8ae862b21b5ebc996`
- **Regression test:** `internal/cli/config_test.go` — `TestConfigRenderingAndMutationHelpers/Should flatten anonymous TOML fields into canonical parent paths` failed on the invalid `roleconfig` segment before the fix and passes after it.

## Verification

- **Retested:** 2026-07-24, Dora / J-route-background-work baseline from a freshly rebuilt and restarted isolated daemon · **Report:** docs/qa/reports/2026-07-24-agent-roles.md
- **Result:** `agh config list -o json` returns `roles.coordinator.enabled`; `agh config get roles.coordinator.enabled -o json` returns `false`; the invalid `roles.coordinator.roleconfig.enabled` path is absent. Package `-race` and repository lint gates pass.
