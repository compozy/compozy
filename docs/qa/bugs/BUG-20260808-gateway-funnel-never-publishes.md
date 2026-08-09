# BUG-20260808-gateway-funnel-never-publishes: Public gateway never becomes reachable on first activation

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-deliver-through-public-gateway, enable the public provider
- **Scenarios:** RT-gateway-public-ingress-bindings
- **Found:** 2026-08-08 · **Report:** docs/qa/reports/2026-08-08-remote-gateway-tailscale-github.md

## Summary

Bruno enables the bundled Tailscale provider, but the public gateway becomes degraded after one
probe and never becomes reachable. The trigger therefore has no live URL and the GitHub delivery
cannot start.

## Reproduction

- **Charter:** CH-gateway-github-delivery · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US, real Tailscale account and bundled `tsnet` provider

1. Enable HTTPS certificates and Funnel access in the Tailscale tailnet.
2. Bind a fresh Tailscale auth key and enable the bundled provider for the public tier.
3. Read `compozy gateway status -o json` after the first verification deadline.

**Expected:** The endpoint remains unadvertised while Compozy waits for public DNS, then becomes live after the public challenge succeeds.
**Actual:** The provider becomes degraded after one probe, its Funnel listener is removed, and no automatic initial recovery can keep the Funnel alive long enough for public DNS to publish.

## Evidence

- `docs/qa/reports/2026-08-08-remote-gateway-tailscale-github.md`
- `.tmp/compozy-dev-home/logs/compozy.log` records repeated `establishing → endpoint.rejected → degraded` transitions with `gateway endpoint unverified: endpoint probe failed` and no advertised address.
- Public DNS returned `NXDOMAIN`; MagicDNS returned the private CGNAT address `100.81.170.88`, which the public SSRF policy correctly rejected.
- After publication, DNS-over-HTTPS returned Funnel IPs `199.38.181.54` and `209.177.145.137` while
  explicit UDP and TCP requests to port 53 still returned an empty answer on the Tailscale-connected
  Mac.
- The Tailscale machine page reported `No certificate found`. After explicit certificate provisioning,
  `tailscaled.log2.txt` showed the ACME DNS update being canceled by the 10-second endpoint-proof
  deadline.

## Fix

- **Root cause:** Three first-activation stages were coupled incorrectly. Public verification used the
  host resolver, which can return a private MagicDNS address, and plaintext DNS remained subject to
  the local Tailscale DNS path; the reconciler tore down an unverified
  Funnel after the first probe; and `tsnet` deferred certificate issuance until a client handshake while
  Compozy used the 10-second endpoint-proof deadline for provider activation. The latter canceled the
  ACME DNS update before the certificate could be issued.
- **Fix commit:** pending completion gate
- **Regression test:** `internal/gateway/endpoint_verify_test.go`,
  `internal/gateway/provider_supervisor_test.go`, `internal/gateway/policy_test.go`, and
  `extensions/connectivity/tailscale/provider_test.go` cover public DNS isolation, staged session
  retention, automatic initial recovery, an independent provider-activation deadline, and certificate
  provisioning before the public listener opens.

## Verification

- **Retested:** 2026-08-08
- **Result:** Pass — the Tailscale machine held a valid certificate, the public challenge returned
  `200`, gateway status reached `up` with `advertised=true`, and GitHub Actions delivery
  `github-31262366045-1` produced exactly one completed Loop.
- **Evidence:** GitHub Actions run `31262366045`; automation run
  `run_wbh_bfa35e72f4aa5b0d2909a010`; Loop run `looprun-3605ec461ab966d7`.
