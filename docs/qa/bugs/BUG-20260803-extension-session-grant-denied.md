# BUG-20260803-extension-session-grant-denied: Valid extension Host API calls are denied after initialize

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada; Bruno
- **Journey Step:** J-extension-distribution, activate an installed extension and use its declared Host API permission
- **Scenarios:** ET-extension-host-api-grant-continuity
- **Found:** 2026-08-03 · **Report:** `docs/qa/reports/2026-08-03-extension-host-api-grant-continuity.md`
- **Origin:** restarted Go package audit, complete tagged Extension integration suite

## Summary

An extension can declare `sessions/list`, receive that permission in its initialize handshake, and
still receive `Capability denied` on the first live Host API call. The failure affects both the Go
and TypeScript reference runtimes and repeats after another install reloads all managed extensions.

## Reproduction

1. Start a fresh daemon and install `secret-guard` from `internal/extension/testdata/secret-guard`.
2. Install the TypeScript `prompt-enhancer` reference and the `clarify-tool` reference extension so
   each mutation reloads the installed runtime set.
3. Confirm the active `secret-guard` and `prompt-enhancer` handshakes include `sessions/list` in
   `granted_permissions`.
4. Observe the first `sessions/list` Host API call from each active process.

**Expected:** Each live process receives a successful session list under its own session-bound grant.
**Actual:** Each process receives JSON-RPC `-32001` / `Capability denied`.

## Evidence

- Red reproduction: `CGO_ENABLED=1 go test -tags=integration -race ./internal/extension -run '^TestReferenceExtensionsEndToEnd$' -count=1 -v`
- Canonical regression: `TestManagerWrapHostHandlerInjectsExtensionNameForHostAPIHandler` failed with JSON-RPC `-32001` when only the live session grant existed.
- Reference runtime contract: `internal/extension/reference_integration_test.go` and `internal/extension/testdata/secret-guard/main.go`.

## Fix

- **Root cause:** Runtime launch registered a nonce-scoped private grant and the Manager wrapper
  authorized with it correctly. The shared `HostAPIHandler` then performed a second permission check
  using the stable public extension name, which intentionally had no session grant.
- **Correction:** The internal Host API context now carries the private capability-grant identity
  separately from the public extension name. Permission checks use the private session identity;
  downstream ownership, bridge authorization, resource attribution, rate limiting, and diagnostics
  continue using the stable name.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** the canonical manager adapter test proves both identities simultaneously, and
  the reference E2E proves real Go and TypeScript subprocess calls across repeated manager reloads.

## Verification

- **Technical result:** fixed. The adapter regression passes 20/20 under `-race`; the reference E2E
  passes 3/3; complete untagged and tagged Extension race suites pass; default/tagged vet, Windows
  amd64 tagged test compilation, zero-issue Go lint, file caps, formatting, and diff checks pass.
- **Public QA result:** verified. A fresh isolated lab installed the Go and TypeScript reference
  extensions through the CLI, observed successful `sessions/list` calls before and after live
  process replacement, confirmed CLI and HTTP status parity, and proved the ungranted
  `sessions/create` method remains denied. The lab teardown reports `clean=true` and no survivors.

## Public QA Evidence

- Focused result: `/home/pedronauck/dev/qa-labs/compozy-extension-host-api-grant-continuity-20260803-034903-293855-lab/qa-artifacts/qa/evidence/extension-host-api/verification.json`
- Strict audit: `/home/pedronauck/dev/qa-labs/compozy-extension-host-api-grant-continuity-20260803-034903-293855-lab/qa-artifacts/qa/qa-audit-report.json`
- Teardown: `/home/pedronauck/dev/qa-labs/compozy-extension-host-api-grant-continuity-20260803-034903-293855-lab/qa-artifacts/qa/teardown.json`
