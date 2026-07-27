# BUG-20260727-ephemeral-role-session-leak: Internal role retries leak failed sessions

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-11 List sessions, role-driven background work
- **Scenarios:** RT-011
- **Found:** 2026-07-27 · **Report:** docs/qa/reports/2026-07-27-devtool-oss-launch.md
- **Origin:** n/a

## Summary

Théo can see failed startup artifacts from provider fallback attempts that were never accepted as
real sessions. Automatic titles, memory extraction, checkpoint summaries, and coordinator startup
can therefore pollute the session catalog when an internal role's first provider is unavailable.

## Reproduction

- **Charter:** CH-compozy-runtime-provenance · **Tour:** Garbage Tour
- **Environment:** isolated macOS lab, desktop / wifi-fast / en-US

1. Configure the automatic-title role with an unreachable primary provider and a healthy fallback.
2. Prompt a normal user session and wait for the role fallback to complete.
3. Read the workspace session catalog from the global store.

**Expected:** The fallback succeeds without persisting the rejected pre-acceptance attempt; only
the user session and accepted role sessions are durable.

**Actual:** The rejected provider attempt is persisted as a stopped startup-failure session. When
every provider is unreachable, each rejected attempt can remain in the catalog.

## Evidence

- Pre-fix integration failure:
  `/Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration.log`
  (`TestAutoTitleRoleIntegration` timed out while waiting for fallback cleanup).
- Post-fix focused integration:
  `/Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/role-session-cleanup-e2e.log`.
- Full tagged-integration rerun:
  `/Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration-rerun.log`.

## Fix

- **Root cause:** Synchronous `Create` persisted every startup failure before returning. That is
  correct for user-created sessions and asynchronously accepted sessions, but internal role
  orchestration reused the same path for retry attempts that had not crossed its acceptance
  boundary.
- **Fix commit:** pending
- **Regression test:** `internal/daemon/auto_title_role_integration_test.go` — rejected fallback
  attempts are absent from the workspace catalog, exhausted attempts are fully cleaned up, and an
  accepted child remains durable if later work fails.

## Verification

- **Retested:** 2026-07-27, same isolated integration envelope · **Report:**
  docs/qa/reports/2026-07-27-devtool-oss-launch.md
- **Result:** Pass. All four automatic-title role cases pass, including healthy fallback, exhausted
  cleanup, and the accepted-child durability boundary; the full 18,552-test integration lane also
  passes.
