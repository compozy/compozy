# BUG-20260801-extension-cli-workspace-reads: Workspace extension is invisible to CLI list and status

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-dev-lifecycle, inspect the linked workspace instance
- **Scenarios:** ET-extension-dev-reload-loop
- **Found:** 2026-08-01 · **Report:** docs/qa/reports/2026-08-01-loops-paper-adoption.md

## Summary

Bruno linked a development extension to a workspace, but the public CLI could not list or inspect
that workspace instance. `extension list` returned only the global inventory and `extension status`
could not select the dev overlay, forcing an agent or operator to leave the CLI and query HTTP
directly before continuing the inner loop.

## Reproduction

- **Charter:** CH-extension-dev-link-invoke · **Tour:** Feature Tour
- **Environment:** isolated local daemon and stamped CLI, desktop / wifi-fast / en-US

1. Register a workspace and link a locally built extension with `compozy extension dev --workspace`.
2. Run `compozy extension list --workspace <workspace> -o json`.
3. Run `compozy extension status <name> --workspace <workspace> -o json`.

**Expected:** Both commands resolve the workspace reference and return the effective dev overlay.

**Actual:** The commands had no workspace flag. The unscoped list returned only the global set and
the unscoped status could not read the workspace-only extension.

## Evidence

- Public replay and before/after command results:
  `docs/qa/reports/2026-08-01-loops-paper-adoption.md`.
- Clean isolated-runtime teardown:
  `/tmp/compozy-loops-postrebase.N1iPVr/teardown.json`.

## Fix

- **Root cause:** The HTTP/UDS extension read service already accepted a workspace query, but the
  CLI `list` and `status` commands exposed only the global client methods and no workspace option.
- **Correction:** Both commands now accept `--workspace`, resolve aliases and paths to the stable
  workspace registration ID, and call the existing scoped read contract. CLI reference docs, the
  development guide, and the official Compozy skill document the same behavior.
- **Fix commit:** 98feabf
- **Regression test:** `internal/cli/extension_test.go` owns command-level workspace resolution in
  `TestExtensionScopedReadsResolveStableWorkspaceID`; `internal/cli/client_test.go` owns the scoped
  HTTP/UDS query projection in `TestUnixSocketClientExtensionMethods`.

## Verification

- **Retested:** 2026-08-01, same persona/journey · **Report:**
  `docs/qa/reports/2026-08-01-loops-paper-adoption.md`
- **Result:** The rebuilt CLI listed and inspected the same workspace overlay by stable ID, invoked
  its tool, observed a new generation after source edit and reload, read logs, removed the link, and
  then confirmed that both list and status no longer exposed the removed overlay.
