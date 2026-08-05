# BUG-20260805-session-archive-sdk-missing: Extension authors cannot use typed session archive helpers

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-archive-session-without-deleting, call archive through an extension Host API client
- **Scenarios:** RT-session-archive-catalog
- **Found:** 2026-08-05 · **Report:** `docs/qa/reports/2026-08-04-session-archive.md`

## Summary

An extension author can receive the `sessions/archive` and `sessions/unarchive` permissions, but the
public Go method constants and TypeScript `HostAPI.sessions` facade omit both operations. The raw
Host API remains callable, but authors lose the documented, type-safe path and must invent string
literals or drop to an untyped request.

## Reproduction

- **Charter:** CH-archive-session-structured-parity · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US

1. Open the public Go SDK `HostAPIMethod` constants and the TypeScript `HostAPI.sessions` facade.
2. Attempt to author an extension that archives and restores a stopped session.
3. Compare the public helpers with the generated `HostAPIMethodMap`, which already includes both methods.

**Expected:** Both SDKs expose the same archive and unarchive operations as the daemon contract.
**Actual:** Only raw string-based requests can reach those two methods.

## Evidence

- `sdk/go/contracts/host_methods_gen.go` and `sdk/typescript/src/generated/contracts.ts` contain the methods.
- `sdk/go/host_api.go` and `sdk/typescript/src/host-api.ts` omit their public helpers before the fix.
- `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/native-archive.json`

## Fix

- **Root cause:** Contract code generation updated the generated method maps, but the hand-authored public SDK facades were not included in the co-ship checklist.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** `sdk/go/runtime_contract_test.go`; `sdk/typescript/src/__tests__/host-api.test.ts`.

## Verification

Pending implementation and the original extension-author journey replay.
