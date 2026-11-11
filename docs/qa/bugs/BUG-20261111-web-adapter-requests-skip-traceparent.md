# BUG-20261111-web-adapter-requests-skip-traceparent: Some web requests to the daemon start their own trace

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Dora
- **Journey Step:** J-export-telemetry-to-collector, "WORK: prompt from CLI, web, or desktop"
- **Scenarios:** OBS-otel-export
- **Found:** 2026-11-11 · **Report:** docs/qa/reports/2026-11-11-otel-telemetry-export.md

## Summary

With export on, Dora opens a session in the web UI. Most daemon requests, including the prompt POST, carry an unsampled
`traceparent` (`00-<id>-<span>-00`), but the agent-instance, terminal, and window-manager (read and client
registration) requests carry none; code reading shows the window-manager layout settings and support-bundle requests
take the same path. Those requests still produce server spans, but
under daemon-chosen trace ids, so the web client cannot correlate them and the "every daemon-bound request carries a
`-00` header" contract (OBS-otel-export §3, `_dx.md` sample-ratio rule) does not hold for them.

## Reproduction

- **Charter:** CH-otel-export-collector-walk · **Tour:** Feature Tour
- **Environment:** desktop browser (agent-browser, Chromium), branch web dev server proxied to the lab daemon

1. Enable export, restart the daemon, open `/session/<id>` in the web UI with a HAR recording.
2. Send a prompt from the composer.
3. List the HAR's `/api/` requests and their `traceparent` header.

**Expected:** every `/api/` request (except the EventSource stream, which cannot set headers) carries
`traceparent: 00-<32 hex>-<16 hex>-00`.
**Actual:** 232 requests carried one; 9 did not: `GET /api/agent-instances`, `GET /api/terminals`,
`GET /api/workspaces/global/window-manager`, `POST /api/workspaces/global/window-manager/clients` (plus 2 SSE
`…/stream` requests, which are out of scope).

## Evidence

- `docs/qa/evidence/2026-11-11-otel-telemetry-export/w3-web-requests.json` (before the fix)
- `docs/qa/evidence/2026-11-11-otel-telemetry-export/w3-web-requests-after-fix.json` (after the fix)

## Fix

- **Root cause:** five adapters (`agent-instance-api`, `terminal-api`, `window-manager-api`,
  `window-manager-layouts-api`, `support-api`) called the daemon with `runtimeFetch`, a bare `globalThis.fetch`
  delegate exported from `web/src/lib/api-client.ts`. Only `apiClient`'s middleware and `daemonFetch` applied the
  traceparent rule, so any adapter that picked `runtimeFetch` silently opted out.
- **Fix commit:** see the report's What Was Fixed (`fix: send traceparent from every web daemon adapter`).
  `runtimeFetch` is removed; `apiClient` and every adapter use `daemonFetch`, the single place that applies the rule.
- **Regression test:** `web/src/systems/terminal/adapters/__tests__/terminal-api.test.ts` "Should join each daemon
  request to a fresh unsampled trace" — failed before (no `traceparent`), passes after.

## Verification

- **Retested:** 2026-11-11, same persona/journey, web walk re-run after the fix · **Report:**
  docs/qa/reports/2026-11-11-otel-telemetry-export.md
