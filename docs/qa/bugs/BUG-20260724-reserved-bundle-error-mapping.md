# BUG-20260724-reserved-bundle-error-mapping: Reserved bundle agents return internal errors

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada; agent operators
- **Journey Step:** J-32 reserved-name bundle materialization probe
- **Scenarios:** RT-reserved-builtin-agent-names
- **Found:** 2026-07-24 · **Report:** docs/qa/reports/2026-07-24-agent-roles.md

## Summary

Once a packaged `coordinator` reached bundle validation, the domain returned the correct `ErrAgentNameReserved` sentinel and wrote nothing. HTTP and UDS nevertheless classified it as 500, and the native adapter collapsed it to `tool_backend_failed` rather than the public `agent_name_reserved` code.

## Reproduction

- **Charter:** CH-reserved-builtin-name-sweep · **Tour:** Garbage Tour
- **Environment:** desktop / wifi-fast / en-US, isolated `devtool-oss-launch` lab

1. Activate a bundle profile that packages an agent named `coordinator`.
2. Repeat through HTTP, UDS/CLI, and `agh__bundles_activate`.
3. Inspect status, diagnostic code, and post-attempt activation/catalog state.

**Expected:** HTTP and UDS return 422, CLI exits with its diagnostic exit class, native returns `agent_name_reserved`, and no state changes.
**Actual:** HTTP and UDS return 500; native classifies the same validation error as a backend failure.

## Evidence

- Before: `reserved-bundle-{http,uds,cli}-valid-probe.*` in the run's `qa-artifacts/qa` directory.
- After: `reserved-bundle-{http,uds,cli}-fixed-probe.*` and `native-bundle-capable-session-history.json`.

## Fix

- **Root cause:** `StatusForBundleError` did not classify `ErrAgentNameReserved`, and the native bundle adapter relied only on the resulting generic HTTP status mapping.
- **Fix:** Bundle API mapping now returns 422 for the sentinel; the native adapter preserves `ErrorCodeAgentNameReserved` with invalid-input/schema semantics.
- **Fix commit:** `a1c966c01b40ae37372e4431704703acd92e679a`
- **Regression tests:** `TestStatusForBundleErrorAndChannelHelpers/agent name reserved` and `TestDaemonNativeTools/Should preserve the reserved agent code from bundle activation`.

## Verification

- CLI/HTTP/UDS returned `agent_name_reserved`; HTTP and UDS returned 422 and named `.agh/bundles/<activation>/agents/coordinator/AGENT.md`.
- A real Codex session invoked `agh__bundles_activate` once and received `agent_name_reserved: Unprocessable Entity`.
- Activation and agent catalogs were byte-stable across every rejected attempt.
- API core passed 1,346 `-race` tests, daemon passed 1,396, and repository lint passed.
