---
title: "Remote gateway: reach your daemon from anywhere"
type: feature
---

A fresh install is still reachable only from the machine it runs on — and now that is a choice
instead of a limitation. The remote gateway adds three independent, off-by-default switches: a
private overlay that serves the full product to devices you pair over your own Tailscale network, a
public delivery ingress that accepts only signed webhook and bridge callbacks, and consent-gated
public operator access for devices that cannot join the overlay. (#331)

- The daemon never binds a public address. Gateway tier listeners stay on loopback and a
  connectivity provider publishes a verified route to them: an address is advertised only after the
  daemon fetches a one-time challenge through it and gets its own nonce back.
- Reaching an address is never authentication. Devices pair with single-use, five-minute artifacts
  written to private `0600` files, credentials are stored only as hashes, and
  `compozy device revoke` cancels live streams before it returns.
- `compozy gateway status|audit`, `compozy pair`, `compozy device`, and `compozy connect` (HTTPS
  profiles plus zero-exposure SSH) operate everything, with the same state in **Settings → Gateway**
  and the `compozy__gateway` native tool.
- Public delivery verifies CompozyOS's timestamped HMAC contract on every request, with replay
  protection and per-source rate limits. There is no store-and-forward while the daemon is offline —
  senders own retries.

Setup guides live in the new Gateway docs section: https://compozy.com/docs/gateway.
