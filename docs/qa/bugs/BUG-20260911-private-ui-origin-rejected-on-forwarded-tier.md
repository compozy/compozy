# BUG-20260911-private-ui-origin-rejected-on-forwarded-tier: On the paired private tier, the operator UI never loads — every app module is refused with "origin not allowed"

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Iris
- **Journey Step:** J-expose-and-pair-gateway, paired operator session (Truthful Surface Tour)
- **Scenarios:** RT-gateway-paired-device; RT-gateway-operator-surface-truth
- **Found:** 2026-09-11 · **Report:** docs/qa/reports/2026-09-11-mobile-surface-truth.md

## Summary

Iris pairs her second device over the verified private address and opens the
pairing link. The page stays blank: the HTML document loads, but every
JavaScript module and stylesheet under `/assets/` is refused with
`403 {"error":"origin not allowed"}`, so the operator UI can never boot on the
private tier. Pairing therefore cannot be completed through the product's own
QR/link flow, and no paired-operator surface exists at all.

The refusal comes from the CORS/browser-protection origin check: the provider
terminates TLS and forwards raw TCP to the daemon-owned loopback listener, so
the daemon sees the request scheme as `http`; the middleware's
forwarded-target allowance compares canonical origins including scheme, and
the browser's same-origin `https://compozy-gateway…:8443` Origin (sent by ES
module loads) never matches. Plain document navigations (no `Origin` header)
load — which is why the blank page shows the served HTML but no app.

## Reproduction

- **Charter:** CH-gateway-paired-operator-surface · **Tour:** Truthful Surface Tour
- **Environment:** authorized tailscale provider, private tier advertised and live; Chromium (Playwright) against the verified private address; also reproduced with serialized single-request loads (not a concurrency artifact)

1. Bring up the private tier with a verified address (`gateway status` shows the live `https://…ts.net:8443` endpoint).
2. Mint a pairing code (Web Settings → Remote access → Pair a device; dialog shows QR + link).
3. On a second device/browser, open the pairing link.
4. Observe a blank page; the console shows `403` for every `/assets/*.js` and `/assets/*.css`.

**Expected:** the pairing link opens the operator UI, which completes the redeem and lands in the paired operator session.
**Actual:** `403 {"error":"origin not allowed"}` for every module/stylesheet; the UI never renders.

## Evidence

- docs/qa/evidence/2026-09-11-mobile-surface-truth/ts-03-second-device-redeem.png (blank page after opening the pairing link)
- docs/qa/evidence/2026-09-11-mobile-surface-truth/ts-02-pairing-with-address.png (the QR + link dialog backed by the verified address)
- Diagnostic capture: failing request carries `origin: https://compozy-gateway…:8443`; response `403`, `content-type: application/json`, `content-length: 30`, body `{"error":"origin not allowed"}`; direct navigation of the same asset URL (no Origin header) returns 200 with the correct body.

## Fix

- **Root cause:** the provider's tier forwarder was a raw-TCP copier, so the daemon saw forwarded requests as plain `http` and the CORS/browser-protection origin allowance (scheme-comparing) refused the browser's same-origin `https` Origin sent by ES module loads.
- **Fix commit:** 17304fe8f (forwarder rewritten as an HTTP-aware reverse proxy forcing truthful `X-Forwarded-Proto: https` / `X-Forwarded-Host`, stripping client-supplied forwarding headers, SSE flush, 502 on unreachable target)
- **Regression test:** `extensions/connectivity/tailscale/provider_test.go` (forwarder header/scheme cases) — failed before the fix, passes after

## Verification

- **Retested:** 2026-09-11, same persona/journey (Iris, paired private-tier operator session), fresh lab `compozy-mobile-surface-truth-verify2-20260911-141455-792161` · **Report:** docs/qa/reports/2026-09-11-mobile-surface-truth.md (Addendum 2)
- **Result:** the paired SPA boots over the verified private address — `networkidle` in 858ms, zero `/assets/*` rejections, only the expected unauthenticated `/api` 401s; the "Pair this device" gate renders pre-filled from the `#pair=` fragment and redeeming lands the full paired operator session (evidence `ts2-01`, `ts2-05`, `ts2-07`)
