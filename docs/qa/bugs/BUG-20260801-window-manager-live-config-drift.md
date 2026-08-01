# BUG-20260801-window-manager-live-config-drift: a live window setting was blocked or hid an unrelated pending restart

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-agent-manage-window-tabs, configure the live window-manager policy
- **Scenarios:** MS-configure-window-manager
- **Found:** 2026-08-01 · **Report:** `docs/qa/reports/2026-08-01-window-tabs.md`

## Summary

An operator changing `window_manager.nav_stack_limit` while another config edit awaited restart could not trust the result. The generic reload path either blocked the live window setting behind the unrelated drift or advanced the wrong active snapshot; `compozy status` then incorrectly reported the config as current.

## Reproduction

- **Charter:** CH-window-tabs-agent-parity · **Tour:** Feature Tour
- **Environment:** isolated local daemon and CLI, desktop, en-US

1. Start the daemon with `defaults.provider = "codex"`.
2. Set `defaults.provider` to `claude` and leave the restart pending.
3. Run `compozy config set window_manager.nav_stack_limit 1`.
4. Read `compozy status`, config apply history, and the active window navigation stack.

**Expected:** The window setting applies live, the provider change stays pending, status requires restart, and two route pushes retain one prior route.
**Actual:** Before the fix, unrelated drift blocked the live mutation or disappeared from status.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-window-tabs-live-apply-status-retest-20260801-115716-306628-lab/qa-artifacts/qa/evidence/config-apply-history.json`
- `/Users/pedronauck/dev/qa-labs/compozy-window-tabs-live-apply-status-retest-20260801-115716-306628-lab/qa-artifacts/qa/evidence/status-pending-restart.json`
- `/Users/pedronauck/dev/qa-labs/compozy-window-tabs-live-apply-status-retest-20260801-115716-306628-lab/qa-artifacts/qa/evidence/nav-stack-limit.json`

## Fix

- **Root cause:** The CLI treated a section-scoped live mutation as a generic full-config reload. Settings also built a live apply from the desired config instead of projecting only Window Manager over the active snapshot, while status hardcoded the config as current.
- **Fix commit:** `d196f3a7`
- **Regression test:** `internal/cli/config_test.go`, `internal/settings/config_apply_service_test.go`, and `internal/api/core/handler_edge_cases_test.go` pass under `-race` and own the CLI route, active projection, and status invariants respectively.

## Verification

- **Retested:** 2026-08-01, same persona/journey · **Report:** `docs/qa/reports/2026-08-01-window-tabs.md`
- **Result:** The live setting applied at generation 1; desired and active hashes remained different; status reported `warn`, `restart_required: true`, and `pending_restart`; the active navigation stack retained exactly one route.
