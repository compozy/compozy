# QA Run Report — 2026-08-08 — Remote Gateway Tailscale GitHub

- **Scope:** PR #331 external verification — one real GitHub push webhook through the bundled Tailscale public gateway into a workspace-owned Compozy Loop
- **Cadence tier:** targeted
- **Build:** `b61f2e791f3b` plus the verified working-tree remediation · **Environment:** operator-provided dev runtime at `http://127.0.0.1:2123`, Web at `http://localhost:3001`, `COMPOZY_HOME=.tmp/compozy-dev-home`, normal authenticated Chrome profile, real GitHub and Tailscale accounts
- **Started:** 2026-08-08T13:33:05Z · **Status:** passed after fixes

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-gateway-github-delivery |

## Flows in Scope

- `J-deliver-through-public-gateway` — connect a repository sender to Compozy and confirm both ends agree (`../journeys/J-deliver-through-public-gateway.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-gateway-github-delivery | J-deliver-through-public-gateway / RT-gateway-public-ingress-bindings; TA-056 | Bruno | Feature Tour | Fixed | BUG-20260808-gateway-funnel-never-publishes | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Bruno enabled the real bundled Tailscale provider, confirmed the workspace trigger binding, and
pushed commit `1b56e60` to a private GitHub repository. GitHub Actions run `31262366045` sent one
Compozy-signed delivery through the public Funnel and succeeded in 6 seconds.

Compozy accepted delivery `github-31262366045-1` with HTTP `200`, created automation run
`run_wbh_bfa35e72f4aa5b0d2909a010`, and delegated exactly one Loop run,
`looprun-3605ec461ab966d7`. The Loop was workspace-scoped to `ws_e83b78ca15d3d733`, preserved the
`github-push-via-tailscale` marker, and finished `done`. The gateway audit reported no findings.

## What Was Fixed

- Public endpoint verification now uses authenticated DNS-over-TLS instead of a host or plaintext DNS
  path that Tailscale can answer with MagicDNS state.
- Failed first-use proof retains the unadvertised provider listener and challenge while bounded recovery
  retries.
- Provider activation has a 60-second bound independent of the 10-second endpoint proof.
- The bundled Tailscale provider provisions its HTTPS certificate before opening the Funnel listener.

## Paper Cuts

- GitHub's file editor produced an extra path segment for the receipt file. It remained below
  `delivery/**`, so the intended single workflow run was unaffected; no second push was made.

## Runtime Errors Observed

- Initial public DNS was absent while the Tailscale certificate had not been issued.
- The first explicit certificate attempt reached the ACME DNS update but was canceled by the coupled
  10-second deadline.
- Direct plaintext DNS on the Tailscale-connected Mac returned an empty MagicDNS answer after public
  DNS-over-HTTPS already exposed the Funnel IPs.

## Human Verifications Needed

None. GitHub required passkey reauthentication to delete the two encrypted Actions secrets, so they
remain in the private evidence repository; both are inert after ingress teardown.

## Decisions for a Human

None.

## Cleanup

- Removed the public ingress binding and disabled the public surface, provider activation, gateway
  ceiling, and bundled Tailscale extension.
- Removed the extension secret binding and the local `tsnet` state containing the issued private key.
- Disabled the temporary Mac Funnel, restored Tailscale DNS acceptance, removed the temporary Funnel
  policy grant, deleted the ephemeral `compozy-gateway` node, and removed the temporary certificate
  directory.
- Kept the private GitHub repository and the local Compozy trigger, Loop, and completed run as durable
  evidence. The operator-provided daemon remained running.

## Learnings

- GitHub's native repository webhook signs `X-Hub-Signature-256`, while the generic Compozy trigger
  intentionally requires its own timestamped `X-Compozy-*` HMAC contract. A GitHub Actions adapter is
  therefore required for this generic trigger; the successful workflow is the real GitHub push event
  and performs that signing at the sender boundary.
- Tailscale's first Funnel activation has three eventually consistent stages: certificate issuance,
  public DNS publication, and public challenge reachability. They need separate bounded lifecycles.

## Final Status

Pass after fixes. GitHub Actions run `31262366045` and delivery `github-31262366045-1` produced one
and only one attributed Loop run, which finished `done`; gateway audit returned `no_findings=true`.
External exposure and temporary Tailscale resources were removed after evidence capture.
