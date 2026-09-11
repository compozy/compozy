---
id: RT-gateway-operator-surface-truth
area: RT
title: Read truthful gateway posture from the operator surface
persona: Iris
journey: J-expose-and-pair-gateway
expected: Gateway settings and structured status agree on named exposure modes, desired and observed state, verified addresses, provider health and cause, refusals, and local-only recovery without presenting an unsupported control or plausible dead URL.
entry_points: Web /settings/gateway; compozy gateway status -o json; HTTP/UDS GET /api/gateway/status
qa_status: pass
bug_ids: BUG-20260807-gateway-live-config-copy;BUG-20260807-gateway-provider-cause;BUG-20260807-gateway-provider-boot;BUG-20260911-private-ui-origin-rejected-on-forwarded-tier;BUG-20260911-private-tier-loopback-guard-inert
fix_status: fixed
retest_status: pass
fix_commits: 17304fe8f
evidence: docs/qa/evidence/2026-09-11-mobile-surface-truth/ts2-08-br1-paired-shell.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/ts2-14-open-terminal-dispatch.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/ts2-15-loopback-strip-after-write.png
last_report: docs/qa/reports/2026-09-11-mobile-surface-truth.md
overlaps: RT-gateway-local-only-boot; RT-connectivity-provider-route
---

Walk fresh, loading, empty, refused, pending, degraded, live, and reconfirmation-required states.
Provider inventory failure must not look like an empty catalog; a degraded row must keep its cause
visible. Each address belongs to its own observed surface and disappears when verification or
admission is withdrawn.

The settings surface must remain keyboard-operable, screen-reader legible, responsive at the
project's supported viewports, and free of color-only state. Runtime truth wins over extension
claims and optimistic mutations.

QA walk 2026-08-07: Web, CLI, HTTP, and UDS agreed on local-only, degraded, refusal, and remediated
states; three production defects were fixed and re-walked. A truthful live-address presentation
remains blocked because the provider account is unavailable.

Re-opened as untested 2026-09-11: the mobile-surface-truth change (branch `mobile-surface-truth`)
altered the operator surface — loopback-only affordances are now absent on remote tiers and a
truthful loopback-only state exists for the two stable 403 codes. The surface-truth walk (including
remote live-address presentation) re-runs under CH-gateway-paired-operator-surface; the real-address
leg remains externally blocked pending an authorized TS_AUTHKEY.

QA walk 2026-09-11 (CH-gateway-paired-operator-surface, lab
`compozy-mobile-surface-truth-20260911-041346-834544`): local surface truth re-passed — Web
Settings → Remote access, `compozy gateway status -o json`, HTTP, and UDS agreed on the same
posture (`gateway.enabled` on, private tier desired=enabled with the provider down, operator_ui
surface desired=enabled/observed=off, "local only · 0 paired devices" header); the degraded
provider row keeps its actionable cause ("Bind TS_AUTHKEY for tailscale in Settings →
Extensions") without inventing an address; the private-overlay card shows the truthful
"Establishing" state instead of a plausible dead URL; the pairing dialog names the missing
verified address and blocks device handoff honestly. Direct loopback hits on the main daemon
latch the `X-Compozy-Gateway-Tier: local` header (full local surface), and the private-tier
listener never binds without the provider (fail-closed, recovery retries), so the new
loopback-only affordance gating and the truthful 403 strip remain unverifiable in-product —
blocked-verify on the named external blocker (authorized `TS_AUTHKEY`).

Remote-leg walk 2026-09-11 with the authorized provider (lab
`compozy-mobile-surface-truth-remote-20260911-125112-264736`): the truthful live-address
presentation VERIFIED in-product — the Reachability card shows "Reachable / Verified: this address
was proven to reach this machine" with the live `https://…ts.net:8443` endpoint (`live: true`),
the provider row reads "Healthy — Up", and Web/CLI/HTTP/UDS agreed on the same posture
(`operator_ui` surface `on`, tier `up`, advertised). The paired operator surface walk itself is
blocked by two filed defects: (1) the UI cannot boot over the private tier — every ES-module asset
is refused `403 {"error":"origin not allowed"}` by the CORS origin check on the forwarded hop
(BUG-20260911-private-ui-origin-rejected-on-forwarded-tier), so BR-1 absence walk, the
deep-linked-write truthful strip (US-002.AC-2 / T8), and the remote session are unreachable;
(2) the server-side enforcement behind those surfaces is absent — with a paired device credential,
guarded mutations (drain/undrain proven 200 with real state change; guarded settings PATCH passed
the guard to handler validation) execute over the private tier instead of refusing
`loopback_mutation_required` (BUG-20260911-private-tier-loopback-guard-inert). Stream-ticket
mechanics verified API-level: mint 201 with TTL, single-use connect (reuse → 401
`gateway_stream_ticket_invalid`), reconnect re-mints, and revocation closed an open stream
(`canceled: 1`) before the credential refused. Verdict `blocked-verify` — fix both bugs, then
re-walk the paired operator session end to end.

Verification walk 2026-09-11 after the fixes (commit `17304fe8f`, lab
`compozy-mobile-surface-truth-verify2-20260911-141455-792161`, fresh authorized key, deleted after
bring-up): both defects verified fixed in-product and the surface-truth walk completed over the
live private tier. P0 fixed: the paired SPA boots over the forwarded TLS endpoint (858ms
`networkidle`, zero `/assets/*` rejections; only the expected unauthenticated `/api` 401s) and even
gates unpaired visitors to the product's own "Pair this device" redeem form. P1 fixed: with a
paired device cookie, `POST /api/drain` and guarded `PATCH /api/settings/general` now refuse
`403 loopback_mutation_required` (drain state confirmed unchanged by a local read) while reads stay
200; the loopback daemon keeps local mutation semantics (drain/undrain 200 — BR-5 canary). BR-1
verified on the paired session: dock Terminal launcher absent, settings save bar absent, profile
creation absent, and dispatching the daemon-owned "Open Terminal" command lands in the truthful
loopback-only window ("This action runs on the machine running CompozyOS — You are connected
through remote access, so this action cannot run from this device."); a deep-linked settings write
(attention toggle) is refused 403 and the shell renders the truthful loopback-only strip (T8 /
US-002.AC-2 in-product). Streams: single-use tickets, reconnect re-mints, revocation closing live
work (`canceled: 1`) all re-verified. Disclosed residuals, none blocking: notification-delivery
toggles on `/settings/attention` still render as live switches on the paired session (they refuse
403 into the truthful strip — the recorded task_03 residual class; the server-side truth holds);
the settings surface's keyboard/screen-reader rows were not re-probed this walk (unchanged
plumbing; 2026-08-07 walk + UI-13 baseline remain the owning evidence); public tier/Funnel stays
out of scope. Verdict **pass** with those named residuals.
