# BUG-20260817-explicit-release-web-assets-sync: Explicit releases cannot synchronize their web asset pin

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-publish-compozy-beta, explicit release plan
- **Scenarios:** REL-release-candidate-plan; REL-beta-channel-contract
- **Found:** 2026-08-17 · **Report:** docs/qa/reports/2026-08-17-electron-shell.md

## Summary

An explicit desktop release could not reliably synchronize and pin the exact daemon-served web
asset version before resolving release identity. The first synchronization path also omitted the
release token fallback required to publish and read the package.

## Reproduction

- **Charter:** CH-electron-channel-publish-repair · **Tour:** Feature Tour
- **Environment:** `release.yml` workflow dispatch on branch `electron`

1. Dispatch an explicit beta release whose web bundle is not yet pinned to a published package.
2. Let the plan stage publish and resolve the synchronized web asset.

**Expected:** The plan publishes one exact web-assets version, commits its pin, and resolves the
release from that immutable commit.

**Actual:** The plan either skipped synchronization or reached it without the usable release token,
so no authoritative release plan could be emitted.

## Evidence

- https://github.com/compozy/compozy/actions/runs/31996473403

## Fix

- **Root cause:** explicit dispatch did not own web-assets synchronization end to end, and the added
  step did not pass `RELEASE_TOKEN` as its authenticated fallback.
- **Fix commits:** `800fbce`; `7846f4b`
- **Regression test:** the release workflow contract requires synchronization before identity
  resolution and passes the release token to the shared publication action.

## Verification

- **Retested:** the beta.20 release plan published and pinned `compozy-web-assets v0.0.139` before the release was removed by explicit decision.
- **Result:** verified for explicit release planning; the retained run receipt is https://github.com/compozy/compozy/actions/runs/32017263255.
