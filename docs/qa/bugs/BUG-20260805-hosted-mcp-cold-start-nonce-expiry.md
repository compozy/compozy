# BUG-20260805-hosted-mcp-cold-start-nonce-expiry: A cold managed session can start without native tools

- **Status:** fixed
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-load-skill-in-managed-session, step 3
- **Scenarios:** ET-managed-session-skill-loading
- **Found:** 2026-08-05 · **Report:** docs/qa/reports/2026-08-05-issue-314-managed-skill-loading.md

## Summary

Ada's first managed Codex session could not load the installed skill because the hosted-tool nonce
started aging before the external ACP provider initialized. A provider launch longer than the
30-second window therefore reached the bind with an already-expired nonce.

## Reproduction

- **Charter:** CH-managed-session-skill-loading · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US, isolated local runtime, uncached Codex ACP package

1. Start a fresh isolated daemon and register a workspace with an installed skill.
2. Start the first managed Codex session before the provider package is cached.
3. Ask the session to load the skill through the managed tool seam.

**Expected:** The native skill tools bind before the session accepts the prompt.
**Actual:** Provider startup took 34 seconds; the 30-second bind nonce expired, and the session reported
that the native skill tools were unavailable.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-issue-314-native-skill-acceptance-20260805-233430-082729-lab/qa-artifacts/qa/provider-attempt.json`
- Runtime log: `mcp: hosted MCP bind failed`, session `sess-b7f170f79152d443`, reason `nonce_expired`
- Clean retry: session `sess-f82c0a035927b149` bound the native tools and completed the same journey.

## Fix

- **Root cause:** Compozy minted and aged the hosted-MCP bind nonce before ACP initialization, even
  though the provider could not bind hosted tools until initialization and session negotiation.
- **Fix commit:** PR #323 remediation commit.
- **Design:** Launch prepares an unarmed record. ACP startup arms it immediately after initialization
  and before `session/new`, so the TTL covers the actual bind window rather than provider bootstrap.
- **Regression tests:** The hosted launcher rejects bind before activation, preserves expiry after
  activation, and ACP startup proves activation occurs after initialize and before session negotiation.

## Verification

- **Retested:** 2026-08-06 in a fresh isolated lab with a wrapper that delayed Codex ACP startup by 35
  seconds.
- **Result:** The first and only attempt passed. Initialize took 36,958 ms, activation completed next,
  `session/new` took 443 ms, and hosted bind succeeded in 79 ms without `nonce_expired` or retry.
- **Session:** `sess-08797076cb99e02f`.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-issue-314-delayed-native-skill-acceptance-20260806-001626-699034-lab/qa-artifacts/qa/provider-attempt.json`.
