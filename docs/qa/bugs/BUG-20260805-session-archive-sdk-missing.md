# BUG-20260805-session-archive-sdk-missing: Extension authors cannot use typed session archive helpers

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-archive-session-without-deleting, call archive through an extension Host API client
- **Scenarios:** RT-session-archive-catalog
- **Found:** 2026-08-05 · **Report:** `docs/qa/reports/2026-08-04-session-archive.md`

## Summary

An extension author could receive the `sessions/archive` and `sessions/unarchive` permissions, but
the public SDK helpers omitted both operations. After those helpers were added, the daemon adapter
still rejected the same calls instead of forwarding them to the archive-capable session manager.

## Reproduction

- **Charter:** CH-archive-session-structured-parity · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US

1. Open the public Go SDK `HostAPIMethod` constants and the TypeScript `HostAPI.sessions` facade.
2. Attempt to author an extension that archives and restores a stopped session.
3. Compare the public helpers with the generated `HostAPIMethodMap`, which already includes both methods.

**Expected:** Both SDKs expose the same archive and unarchive operations as the daemon contract.
**Actual:** The typed helpers were absent, and a raw Host API call reached an adapter that returned an
internal error instead of archiving the session.

## Evidence

- `sdk/go/contracts/host_methods_gen.go` and `sdk/typescript/src/generated/contracts.ts` contain the methods.
- `sdk/go/host_api.go` and `sdk/typescript/src/host-api.ts` omit their public helpers before the fix.
- `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/notes/extension-host-api-first-attempt-copy.txt`
- `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/extension-host-api.json`

## Fix

- **Root cause:** Contract code generation updated the generated method maps, but the hand-authored public SDK facades were not included in the co-ship checklist. Separately, the daemon's extension adapter exposed only the base session manager interface and discarded archive operations implemented by the concrete manager.
- **Fix commit:** e40dc76.
- **Regression test:** `sdk/go/runtime_contract_test.go`; `sdk/typescript/src/__tests__/host-api.test.ts`; `internal/daemon/daemon_test.go` (`TestNewHostAPISessionManagerAdapter`).

## Verification

- **Retested:** 2026-08-05, same persona/journey · **Report:** `docs/qa/reports/2026-08-04-session-archive.md`
- **Result:** A live installed extension archived, listed, and restored the same stopped session through the typed Host API while preserving its workspace identity.
