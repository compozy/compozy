# QA Run Report — 2026-08-10 — local-stream-auth-clean

- **Scope:** Local Web and desktop-shell stream authorization after replacing remote ticket-route probing with explicit listener-tier discovery.
- **Cadence tier:** targeted
- **Build:** `50ecbccc` + working tree · **Environment:** isolated daemon `http://127.0.0.1:56203`; Web proxy and native desktop shell share the same runtime
- **Started:** 2026-08-10T16:12:45-03:00 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-local-stream-auth-clean |

## Flows in Scope

- `J-operate-desktop-shell` — Reach and arrange local work without duplicate windows, lost context, or invented connection state (`../journeys/J-operate-desktop-shell.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-local-stream-auth-clean | J-operate-desktop-shell / ET-web-desktop-shell-lifecycle | Bruno | Feature Tour | Pass | | working tree |
| 2 | CH-local-stream-auth-clean | J-operate-desktop-shell / ET-web-window-routing-lifecycle | Bruno | Feature Tour | Pass | | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- In an extension-free Chrome profile, Bruno completed setup, opened Loops, selected `software-factory`, and reloaded the detail route. Both Loop endpoints returned 200, the browser recorded no failed response, no console error or warning, and no request to `/api/gateway/stream-tickets`.
- In the native shell, Bruno moved from the Loop detail to the catalog and opened `software-factory` again. The route remained `/loops/software-factory`; the captured daemon window contained 54 HTTP responses, all 200, including the Loop detail and config endpoints, with no ticket request.
- Evidence: `../evidence/2026-08-10-local-stream-auth-clean/browser-web-evidence.json`, `../evidence/2026-08-10-local-stream-auth-clean/software-factory-web.png`, `../evidence/2026-08-10-local-stream-auth-clean/desktop-network-summary.json`, and `../evidence/2026-08-10-local-stream-auth-clean/software-factory-desktop.png`.

## What Was Fixed

- Local Web stream authorization now discovers the listener tier from `X-Compozy-Gateway-Tier` on `/api/status`; it does not probe the remote-only ticket route.
- HTTP responses identify the local, private, or public listener tier, and CORS exposes that header to Web clients.
- The external `imersao-aovivo` `software-factory` Loop now shell-quotes the interpolated task glob, so its detail and config endpoints compile and return 200.

## Paper Cuts

None in the scoped journey.

## Runtime Errors Observed

No Compozy-owned runtime, browser-console, or HTTP error occurred in either verdict window. The reported `contentscript.js` listener and multiplex warnings were absent in the extension-free profile, confirming they are injected by a browser extension rather than emitted by CompozyOS. Expected Vite proxy disconnects occurred only while the isolated daemon was intentionally restarted between the Web and desktop passes.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

None identified yet.

## Learnings

- An explicit transport capability signal removes a noisy control-flow request and makes local versus remote behavior directly testable.
- Extension-free browser evidence is necessary when console messages originate from injected `contentscript.js` code.

## Final Status

**Verdict: PASS.** Both user surfaces completed the scoped journey with clean product console/network evidence. Canonical teardown evidence is emitted to the isolated lab's `qa/teardown.json`; the final repository gate runs after this report is frozen and is recorded in the lab journey log.
