# BUG-20260812-global-workspace-gateway-config: Global Gateway config blocks the operator-home workspace

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Dora
- **Journey Step:** J-expose-and-pair-gateway, step 2
- **Scenarios:** MS-gateway-config-ceiling; RT-gateway-local-only-boot
- **Found:** 2026-08-12 · **Report:** `docs/qa/reports/2026-08-12-pr-webhook-release-notes.md`

## Summary

When Dora kept the operator home registered as a workspace, the daemon interpreted the same
`~/.compozy/config.toml` file first as global configuration and then as a workspace overlay. A valid
global `[gateway]` section was rejected as workspace-only input, so the daemon could not start or
apply a live Gateway setting.

## Reproduction

- **Charter:** CH-gateway-global-workspace-config · **Tour:** Feature Tour
- **Environment:** desktop / operator home and project workspace / wifi-fast / en-US

1. Keep `/Users/pedronauck` registered as a workspace and use `/Users/pedronauck/.compozy` as the
   operator home.
2. Set `gateway.enabled=true` in the global config.
3. Start the daemon from the Compozy workspace or run a global Gateway config write.

**Expected:** The single global file is applied once with global semantics; both workspaces remain
registered and the daemon reaches local readiness.
**Actual:** Startup or the live write failed with `gateway settings are global-only`.

## Evidence

- `internal/config/gateway_test.go` reproduced the loader failure before the fix.
- `internal/config/persistence_integration_test.go` reproduced the global write failure before the fix.
- `docs/qa/reports/2026-08-12-pr-webhook-release-notes.md` records the live operator-home re-walk.

## Fix

- **Root cause:** The loader and persistence tree treated identical canonical global and workspace
  config paths as two configuration layers.
- **Fix commit:** working tree, pending operator commit
- **Regression test:** `internal/config/gateway_test.go` and
  `internal/config/persistence_integration_test.go` failed before the production fix and pass after it.

## Verification

- **Retested:** 2026-08-12, same persona/journey · **Report:**
  `docs/qa/reports/2026-08-12-pr-webhook-release-notes.md`
- **Result:** The daemon started from the project workspace with the operator home still registered,
  structured status reported the global config as current, and a global `gateway.enabled=true` write
  applied live without a workspace-overlay error.
