---
title: Bundled Tailscale connectivity extension
type: feature
---

Gateway reachability ships with a first-party provider. The `tailscale` extension runs a Tailscale
node inside the CompozyOS process through `tsnet`, against the operator's own account — nothing else
to install, and CompozyOS operates no relay, server, or account on anyone's behalf. The private tier
serves `https://compozy-gateway.<tailnet>.ts.net:8443` on the tailnet; the public tier serves the
same hostname over Tailscale Funnel on 443. (#331)

- Bind the auth key once with `compozy extension secrets set tailscale --env TS_AUTHKEY` (hidden
  input); the value never appears in output, status, or diagnostics.
- The extension declares required Live network participation for `gateway.private` and
  `gateway.public`, so enabling asks for a one-time digest confirmation — and asks again only when
  that declaration changes.
- First activation provisions the HTTPS certificate before the Funnel listener opens, verifies
  public endpoints through authenticated DNS-over-TLS (`gateway.verify.public_dns_resolver`), and
  keeps unverified listeners staged with bounded retries instead of tearing them down.
- Third-party providers implement the same `connectivity.provider` contract from the Go and
  TypeScript SDKs, gated by install-source trust and control-digest re-confirmation on every enable
  and boot.
