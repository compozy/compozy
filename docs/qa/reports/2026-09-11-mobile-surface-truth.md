# QA Run Report — 2026-09-11 — Mobile Surface Truth

- **Scope:** `mobile-surface-truth` workflow (branch `mobile-surface-truth`) — truthful operator-surface gating on gateway tiers and mobile (390×844) shell ergonomics; E2E-001 + E2E-002 tail QA pair
- **Cadence tier:** targeted
- **Build:** `029c3773a` (HEAD, clean tree) · **Environment:** isolated lab `compozy-mobile-surface-truth-20260911-041346-834544` — daemon `http://127.0.0.1:53001` serving the built web bundle (`COMPOZY_WEB_DIST_DIR`, production parity), lab `COMPOZY_HOME` `/tmp/compozyqa-4dc542177684/runtime`, UDS socket per manifest; browser driver: Playwright 1.62.1 Chromium (repo-installed) — `agent-browser`/`browser-use` CLIs unavailable in this environment
- **Started:** 2026-09-11T04:15Z · **Status:** closed <!-- in-progress | closed -->

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Marina | Power User | phone 390×844 (device-emulated viewport, touch) / wifi-fast / en-US | CH-mobile-shell-touch-tier |
| Iris | Power User | laptop (desktop 1440×900) / wifi-fast / en-US | CH-gateway-paired-operator-surface |

## Flows in Scope

- `J-expose-and-pair-gateway` — paired private-tier operator sees only executable affordances; device lifecycle local legs (scope per CH-gateway-paired-operator-surface; journey file not yet materialized under `journeys/`)
- `J-operate-desktop-shell` — the operator shell is operable at phone size without lying about state (scope per CH-mobile-shell-touch-tier and the frozen T1–T8 artboard)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-gateway-paired-operator-surface | J-expose-and-pair-gateway / RT-gateway-paired-device | Iris | Truthful Surface Tour | Pass | BUG-20260911-private-ui-origin-rejected-on-forwarded-tier (fixed+verified) | 17304fe8f |
| 2 | CH-gateway-paired-operator-surface | J-expose-and-pair-gateway / RT-gateway-operator-surface-truth | Iris | Truthful Surface Tour | Pass | BUG-20260911-private-tier-loopback-guard-inert (fixed+verified) | 17304fe8f |
| 3 | CH-mobile-shell-touch-tier | J-operate-desktop-shell / APP-mobile-touch-tier | Marina | Touch Tier Tour | Fail | BUG-20260911-session-rail-docks-on-touch-tier | |

Status legend: `Pending | Pass | Fixed | Fail | Skipped | Blocked (needs human verify) | Blocked (human decision)` — rows 1–2 reached `Pass` via the addendum-2 verification walk (their earlier `fail`/`blocked-verify` history lives in the scenario bodies and Addendum 1); row 3 remains `Fail` (different charter, fix pending).

> Row 3 updated by the repair re-walk addendum at the end of this report.

## Session Debriefs

### CH-gateway-paired-operator-surface — Iris

- **Ran:** 2026-09-11T04:45Z → 05:35Z (box respected: yes)
- **Entry:** lab daemon operator UI at 1440×900 (local host), Settings → Remote access, plus the documented HTTP/UDS/CLI structured planes.
- **Findings:**
  - **Honest discovery (the session's first question):** the private-tier listener does **not** answer on its loopback bind without an authorized provider. The reconciler preflights/establishes the connectivity provider *before* binding; with the bundled tailscale extension active but `TS_AUTHKEY` missing, enable fails-closed, recovery retries keep the tier `observed: down` and the `operator_ui` surface `observed: off`, and no listener socket ever exists (`ss` shows only the main daemon on 53001). Admission therefore cannot be probed on the private tier in-lab. On the main daemon, a direct loopback hit latches `X-Compozy-Gateway-Tier: local` (full local surface, no device auth consulted).
  - **Pairing mechanics (local legs) re-passed on this build:** the Web pairing dialog mints a one-time code with expiry and truthful copy — "There is no verified private address yet, so the other device has nowhere to open…" — no QR, no plausible dead URL (RT-gateway-operator-surface-truth contract held). HTTP mint returned `{artifact, expires_at}`; documented redeem (`POST /api/gateway/pairings/redeem`, `{"artifact","name","actor_kind":"operator_device"}`) returned 200 with a `Secure; HttpOnly; SameSite=Lax` `compozy_gateway_device` cookie. `compozy pair redeem` (CLI profile path) truthfully refuses a non-HTTPS origin.
  - **Device inventory cross-surface agreement:** Web ("1 paired device", row with origin + last-active) == HTTP == UDS == `compozy device list -o json` for the same device id. Rename via CLI landed (UI pencil affordance present). Revoke via Web confirm dialog (truthful copy: session closed immediately, streams closed, cannot be undone) and via CLI; `revoke_epoch` bumped to 1 with `revoked_at` set; structured planes retain the marked record while the Web shows "0 paired devices" — consistent active-count semantics (2026-08-07 behavior preserved).
  - **Surface-truth states re-passed:** Settings → Remote access, `compozy gateway status`, HTTP, and UDS agreed on the same posture; the degraded provider row keeps its actionable cause ("Bind TS_AUTHKEY for tailscale in Settings → Extensions") and shows `Down` without inventing an address; the private-overlay card shows the truthful "Establishing" state.
  - **BR-5 local regression walk:** dock Terminal launcher present; palette Terminal section present ("Open Terminal"); settings save surfaces render their normal mutable chrome; notifications/attention switches present (toggles, not read-only pills); profile creation present; tasks surface opens. No affordance regression observed on the local desktop host from the mobile-surface-truth change.
  - **Stream tickets:** not exercisable — the `POST /api/gateway/stream-tickets` route is registered on gateway-tier listeners only, and no gateway listener exists in-lab (above). Recorded as part of the blocked-verify set.
- **Bugs filed/updated:** none (no new defects on the walked legs).
- **Scenarios settled:** RT-gateway-paired-device → blocked-verify; RT-gateway-operator-surface-truth → blocked-verify.
- **Paper cuts:** the structured device list retains revoked records (with `revoked_at`) while the Web shows 0 paired devices — machine-readable and consistent, but a consumer filtering only by name may miscount (dull).
- **Surprises:** `gateway.surface enable` / `provider enable` transitions surfaced `gateway_generation_conflict` until the expected `--generation` was supplied; the error is correct (optimistic-concurrency fence) but the CLI could name the expected generation in the failure (dull).
- **Suggested next charter:** with an authorized `TS_AUTHKEY` provisioned: paired private-tier operator session (absent terminal/write affordances, deep-linked write → truthful loopback-only strip), fresh stream tickets, live revocation during a stream, and the public consent/ingress legs.
- **goal_reached:** partial · **true_end_state:** blocked (remote-tier legs require the user-provisioned authorized `TS_AUTHKEY`; all local legs walked to their end state)

### CH-mobile-shell-touch-tier — Marina

- **Ran:** 2026-09-11T04:20Z → 05:30Z (box respected: yes)
- **Entry:** lab daemon operator UI at 390×844 (device-emulated, DSF 3, touch), onboarding completed through the product's own wizard (model catalog loaded live; operator's env-var API-key path available; workspace registered through the in-product folder browser).
- **Findings (vs the frozen T1–T8 map):**
  - **T1 PASS** — menubar 44px tall; logo/scope/attention/palette/settings controls all exactly 44×44; tab-bar items 44×44 in a 56px bar; traffic lights 44×44 targets in the session window head; palette rows exactly 44px; no body scroll at 390×844 (`scrollHeight == clientHeight == 844`).
  - **T2 PASS** — compact presentation (<960px): dock strip replaced by the 56px tab bar, windows stacked, compact traffic lights; desktop dock returns at 1440×900.
  - **T3 PASS** — window content ends at the tab-bar top in portrait and landscape (work area reserves 56px, not the 82px dock band); nothing renders under the tab bar.
  - **T4 PASS (with parity note)** — viewport meta is exactly `width=device-width, initial-scale=1.0, viewport-fit=cover, interactive-widget=resizes-content`; simulating the keyboard-open visual viewport (548px line from artboard board 02) keeps the composer and tab bar above the bottom edge via flex compression. Headless desktop Chromium has no virtual keyboard, so the `resizes-content` behavior itself was verified by viewport-shrink emulation + the meta contract; a real-device keyboard-open remains a human-verification residual.
  - **T5 PASS** — palette top-anchored (dialog top 76px, below the 44px menubar); rows 44px; results well max-height exactly `min(52dvh, 440px)` = 438.88px at 844px viewport height.
  - **T6 FAIL — BUG-20260911-session-rail-docks-on-touch-tier** — at 390×844 the sessions rail docks as a solid 264px column instead of overlaying the transcript: the transcript/composer compress into the remaining 126px and the composer editor measures 68px wide ("Send a…" clipped, controls stacked). The frozen map promises the overlay + full-width transcript at ≤760px. At 844×390 the same rail docks correctly (>760px desktop rule), isolating the defect to the touch tier.
  - **T7 PASS** — landscape 844×390 keeps the same chrome (menubar 44 + tab bar 56–57, stack ≈289px); height flexes; no window under the tab bar; no landscape-specific chrome.
  - **T8 BLOCKED-VERIFY (in-product)** — the loopback-only strip cannot be triggered in-lab (no remote tier without the authorized provider); placement/touch-floor evidence stays with UT-008 (component suite, reused) and the artboard.
  - **Residuals:** profile-switcher trigger 28×28 well **confirmed** at 390 (and 1440) exactly as recorded in the artboard (dull, cross-system slot); log bodies' horizontal scroll wells **not reached** (extensions log view not opened within the box); scroll-to-bottom pill **not reached** (no scrollable stream: the lab's session truthfully failed to start — "Claude model claude-sonnet-5 is not advertised by the live ACP catalog" — because this machine has no provider CLI binary; no fabricated provider spend was attempted).
  - **Desktop >1024px spot-check PASS** — 1440×900 renders the pre-change shape (44px menubar, floating dock, no tab bar); consistent with UI-13 for unchanged surfaces. Zero console errors and zero page errors across all walks.
- **Bugs filed/updated:** BUG-20260911-session-rail-docks-on-touch-tier (Friction/Medium/P2, linked to APP-mobile-touch-tier).
- **Scenarios settled:** APP-mobile-touch-tier → fail (fix pending).
- **Paper cuts:** session-window header controls are 26×26 at the touch tier (below the 44px floor) — this matches the artboard's win-head grammar (only menubar/dock/palette/scroll pill are frozen to the floor), but Marina with one hand reaches them poorly (dull, watching); a one-off invisible dialog scrim swallowed a single automated click after a rapid palette re-open/Escape cycle — not reproducible in a deliberate clean retry (dull, watching).
- **Surprises:** the onboarding wizard cannot complete in a credential-free lab and is not Escape-dismissable — correct product behavior, but worth knowing for lab bootstrap (the operator's env-var API-key path or a provider CLI sign-in is required).
- **Suggested next charter:** re-walk T6 (rail overlay) after the fix, then the scroll-pill and live-stream legs with a working provider, on a real device for the keyboard-open contract.
- **goal_reached:** yes (all reachable rows walked) · **true_end_state:** confirmed for T1–T5/T7 + desktop; T6 failed with evidence; T8/live-stream legs blocked (no remote tier / no provider CLI in-lab)

## What Was Fixed

None this run. BUG-20260911-session-rail-docks-on-touch-tier is recorded with evidence and left to the loop controller (production web change in `systems/os/apps/session/session-window-content.tsx` composition — outside this session's lane).

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Marina | J-operate-desktop-shell, session header controls | "These header buttons are tiny for my thumb" | dull | watching (matches artboard grammar) |
| Marina | J-operate-desktop-shell, palette rapid re-open | "The first tap after closing the palette did nothing once" | dull | not reproducible clean; watching |
| Iris | J-expose-and-pair-gateway, device inventory | "The list still shows the device I revoked" | dull | structured planes keep `revoked_at` marker; Web shows 0 paired |

## Runtime Errors Observed

- Session start failed truthfully after ~1s: `Claude model "claude-sonnet-5" is not advertised by the live ACP catalog: Provider configuration is unavailable` — the lab machine has no provider CLI binary installed; expected fail-closed in this environment, no retry loop, no fabricated provider spend. Evidence: `i-…` walk + `m-22-session-started.png`.
- Gateway reconcile failures (provider degraded) surfaced only through status `refusal`/generation fields, not the daemon log — observed during lab bring-up, consistent with fail-closed design (dull observability note).
- Zero browser console errors / page errors across all walks (portrait, landscape, palette, settings, remote access, tasks).

## Human Verifications Needed

- [x] ~~Provision an authorized `TS_AUTHKEY` and walk the private-tier legs~~ — **done 2026-09-11** (Addendum 1 + Addendum 2): provider established, pairing/admission/streams/revocation and the BR-1 surface-truth walk all verified in-product; both forwarded-tier defects fixed and verified. **The user should revoke both keys used today.**
- [ ] On a real phone (or device-emulated mobile browser with virtual keyboard), verify the keyboard-open `interactive-widget=resizes-content` contract: composer and palette input stay above the keyboard (row 3 residual).
- [ ] After BUG-20260911-session-rail-docks-on-touch-tier is fixed: re-walk the T6 rail overlay at 390×844, plus the scroll-pill and log-body scroll-well residuals that were not reachable in the morning run (row 3).

## Decisions for a Human

### Session rail docks instead of overlaying at the touch tier (BUG-20260911-session-rail-docks-on-touch-tier)
- What's broken: at 390×844 the sessions rail docks and crushes the composer to a 68px editor, contradicting frozen decision T6 (overlay + full-width transcript). Evidence: `m-23-rail-open-390.png`, `m-26-rail-portrait-evidence.png`.
- Why not auto-fixed: production web change in the task_04 composition wrapper (`session-window-content.tsx` touch-tier branch not applying at ≤760px) — outside this session's lane; the fix-loop governor bounds auto-fixes to clearly-lane-owned, cheap repairs.
- Options: 1. Fix the touch-tier branch in the rail composition wrapper and add a regression test at the ≤760px tier (small, contained — recommended). 2. Re-freeze the artboard to the docked behavior (changes the approved design contract; not recommended).
- Recommendation: option 1, then re-walk the T6 row in a fresh session.

## Learnings

- The gateway reconciler binds the tier listener only *after* provider preflight/establish succeed — a degraded provider means **no listener at all**, even on loopback. In-lab private-tier walks are impossible without a real provider; the 2026-08-07 "gateway states walked locally" evidence was posture/state walks, not listener admission.
- `gateway.surface/provider enable` CLI transitions enforce optimistic concurrency: pass `--generation` from the current status or the persist step returns `gateway_generation_conflict`.
- The lab's operator UI is served by the daemon itself (`COMPOZY_WEB_DIST_DIR`) — the vite dev server is unnecessary for walks; onboarding is completed through the product wizard (model picker → env-var API-key path → workspace folder browser).
- Playwright (via `@playwright/test` in `web/node_modules`) is a workable `agent-browser` substitute for evidence at device-emulated viewports when the browser CLIs are absent; record the driver substitution in the report environment line.
- The redeem endpoint is served on the local listener too; `actor_kind=operator_device` yields the HttpOnly+Secure device cookie, while `actor_kind=cli_profile` is rejected from browser-shaped requests, and the CLI profile path requires an HTTPS origin.

## Final Status

- **Exit gate:** delivery gates intentionally not run by this task (loop controller owns them). Reused gate evidence: `make gate` PASS records under `.cache/gate/` (iterations 2–5, task_05) cover go-lint/test and web lint/typecheck/test for this branch; no code was changed by this QA run.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 1 · Cosmetic 0
- **Coverage:** 2/2 charters walked to a recorded verdict; 3/3 in-scope scenarios settled (1 fail, 2 blocked-verify); T8, stream-ticket, remote-admission, and live-stream legs disclosed as not reachable in-lab (named blockers).
- **Verdict: not ready** — the changed operator surface is truthful locally and the touch-tier chrome holds except T6: fix and re-walk `BUG-20260911-session-rail-docks-on-touch-tier`; the remote-tier legs stay blocked-verify pending the user-provisioned authorized `TS_AUTHKEY`.

---

## Addendum — 2026-09-11 (later): remote-leg walk with authorized provider

- **Scope:** the previously blocked remote-tier legs of `CH-gateway-paired-operator-surface` (Truthful Surface Tour), after the user provisioned an authorized Tailscale auth key. Branch renamed `mobile-surface-truth`; build rebuilt at HEAD `b74d29927` (daemon + web bundle).
- **Environment:** fresh isolated lab `compozy-mobile-surface-truth-remote-20260911-125112-264736` — daemon `http://127.0.0.1:32821`, lab `COMPOZY_HOME` `/tmp/compozyqa-924e1a6ed906/runtime`; bundled tailscale extension installed with network confirmation recorded; `TS_AUTHKEY` bound through the product's extension-secret vault (`extension secrets set … --value-stdin`; never echoed, never persisted in logs or evidence — verified by byte-scan). Browser driver: Playwright 1.62.1 Chromium as before.
- **Secret handling:** the key was read only into the lab daemon's environment and the extension vault binding; `/tmp/opencode/ts-authkey` was deleted after the walk; device-credential cookie jars were deleted with it; no log, evidence file, report, or memory file contains the key or the issued device credentials (byte-level scan performed).
- **Teardown:** manifest `TEARDOWN_COMMAND` executed with `clean: true`; the lab runtime home was additionally purged (`PURGE=1`) because it held the secret binding and tailscale node state.

### Bring-up recipe deltas vs the blocked-run expectations

1. `--digest` on `gateway provider enable` must be the extension's **network-participation requirement digest** (`c014…`, echoed by the install confirmation), not the extension.json checksum.
2. `--source` must be the registry's own source string (`user` for a local-path install), not a filesystem path — otherwise reconcile fails `gateway provider trust stale` and the provider never establishes.
3. The daemon env alone satisfies `missing_env` but the provider subprocess only receives `TS_AUTHKEY` through the **extension secret binding**; after binding, bounce the extension (`disable` → `enable`) so the subprocess re-reads it.
4. Provider establishment needs several recovery cycles (`observed: down → establishing → degraded → up`); `gateway status` is the wait signal.

### Per-leg outcomes

| Leg (must-try) | Outcome |
|---|---|
| Verified private address advertised and reachable | **Verified** — provider `up/healthy`; `addresses[0].live: true` (`https://compozy-gateway…ts.net:8443`); unauthenticated `GET /api/gateway/status` → 401 `gateway_device_unauthenticated` with `X-Compozy-Gateway-Tier: private`; UI document serves 200 |
| Mint pairing with QR + copyable link | **Verified** — dialog shows the QR and the full link `https://…ts.net:8443/#pair=cpz_gwp_…` with expiry; posture card reads "Reachable — Verified: this address was proven to reach this machine" |
| Redeem over the private endpoint → paired session | **Split** — redeem verified through the documented API (`actor_kind=operator_device` → 200 + `Secure; HttpOnly; SameSite=Lax` cookie; reuse → 409 `gateway_pairing_spent`); the **link-open UI path is broken**: the page boots blank because every `/assets/*` ES-module request is refused `403 {"error":"origin not allowed"}` → `BUG-20260911-private-ui-origin-rejected-on-forwarded-tier` (Blocks-Completion) |
| BR-1 walk on the paired remote session (absent affordances, read-only views, truthful loopback-only strip / T8) | **Blocked (in-product defect)** — the paired UI cannot boot (origin bug above); the UI-level walk is unreachable until it is fixed |
| **Server-side enforcement behind those surfaces (BR-6)** | **FAILED — new defect** — with the paired device credential, guarded mutations execute over the private tier: `POST /api/drain` → 200 with real state change (`admission_closed: true`, confirmed by an independent local read; restored via remote `undrain` 200), and a guarded `PATCH /api/settings/general` passed the loopback mutation guard into handler validation instead of refusing `loopback_mutation_required`. The private-tier listener is by design bound to `127.0.0.1`, which makes the bind-host-keyed guard inert → `BUG-20260911-private-tier-loopback-guard-inert` (Trust-Damage, High/P1) |
| Streams: single-use tickets, reconnect re-mints | **Verified (API-level)** — ticket mint 201 with TTL; connect consumed the ticket (reuse → 401 `gateway_stream_ticket_invalid`); re-mint issues a fresh ticket |
| Device inventory agreement across Web/HTTP/UDS/CLI | **Verified** — same device id/name across all four surfaces plus a device-authenticated read over the private endpoint |
| Revocation closes live work, then rejects the credential | **Verified** — `device revoke` closed an open SSE catalog-stream mid-flight (`canceled: 1`; curl exited immediately, not at timeout) and the cookie then refused 401 `gateway_device_unauthenticated` |
| Local BR-5 regression canary on the new build | **Verified** — local shell, onboarding, settings, and Remote access pages render normally on `b74d29927` |

### New bugs filed this addendum

- `BUG-20260911-private-ui-origin-rejected-on-forwarded-tier` — Blocks-Completion/Critical/P0: paired private-tier operator UI never loads (origin check rejects ES-module requests on the forwarded hop).
- `BUG-20260911-private-tier-loopback-guard-inert` — Trust-Damage/High/P1: paired devices can execute guarded daemon mutations (drain/undrain, settings) over the private tier; the loopback mutation guard keys on the loopback bind and never refuses remote paired clients.

### Addendum verdicts

- `RT-gateway-paired-device` → **fail** (fix pending): lifecycle + admission + revocation verified over the real private tier, but the product's own QR/link pairing flow dead-ends in a blank page (origin bug); both new bugs linked.
- `RT-gateway-operator-surface-truth` → **blocked-verify** (fix pending): live-address presentation now verified in-product; the paired operator surface walk (BR-1 absences, truthful loopback-only strip/T8) is unreachable behind the origin bug, and the enforcement behind those surfaces is absent (guard bug). Both linked.

### Addendum final status

- **Issues by user impact (addendum):** Blocks-Completion 1 · Data-Loss 0 · Trust-Damage 1 · Friction 0 · Cosmetic 0
- **Secret disposition:** authorized `TS_AUTHKEY` deleted from `/tmp/opencode/ts-authkey` after the walk; zero occurrences in any persisted artifact (byte-scan); lab runtime purged. The user should revoke the key at the tailscale tailnet as it was live during the session.
- **Verdict (addendum): not ready** — the remote-tier story is architecturally alive (provider establishes, pairing/admission/streams/revocation all behave), but two P0/P1-class defects (UI origin refusal; missing server-side enforcement) must be fixed and the paired session re-walked before the surface-truth promise holds.

---

## Addendum 2 — 2026-09-11 (verification walk): fixes for the forwarded-tier defects verified in-product

- **Scope:** in-product verification that `17304fe8f` fixes `BUG-20260911-private-ui-origin-rejected-on-forwarded-tier` (P0) and `BUG-20260911-private-tier-loopback-guard-inert` (P1), and completion of the blocked BR-1 legs of `CH-gateway-paired-operator-surface`.
- **Environment:** fresh lab `compozy-mobile-surface-truth-verify2-20260911-141455-792161` — daemon `http://127.0.0.1:35569`, lab `COMPOZY_HOME` `/tmp/compozyqa-0fa9adbe4836/runtime`, build at HEAD (`17304fe8f` + QA docs). Fresh authorized key used for bring-up only: bound via the extension vault, **secret file deleted immediately after the tier reached `up`**; zero key-material occurrences by byte-scan (daemon log, docs, memory, lab artifacts); device-cookie temp jars deleted after the walk; all five walk devices revoked at the end (empty-inventory recovery root confirmed).
- **Bring-up note (recipe unchanged from the previous addendum, one delta):** the provider-only enable transition does not by itself drive the first establish attempt; the `operator_ui` surface enable does (it errored `gateway provider degraded` once — tsnet still negotiating — then recovery established the tier: `down → establishing → up` with a verified live address).

### Fix verification measurements

| Defect | Expected after fix | Observed |
|---|---|---|
| P0 — origin rejection on the forwarded tier | paired SPA boots over `https://…ts.net:8443`; no `/assets/*` 403s | **Verified** — `networkidle` in 858ms; zero asset rejections across portrait/desktop loads; the only 4xx are the expected `401` on unauthenticated `/api/*`; the app renders its own "Pair this device" gate, pre-filled from the `#pair=` fragment; redeem completes in-UI → paired operator session with the Secure/HttpOnly cookie |
| P1 — loopback guard inert on the private tier | paired device cookie: guarded mutations → 403 `loopback_mutation_required`; reads → 200; local loopback keeps mutation semantics | **Verified** — `POST /api/drain` → 403 `loopback_mutation_required` (drain state unchanged, confirmed by local read); guarded `PATCH /api/settings/general` → 403 same code; `GET /api/gateway/status` (tier `private`), `GET /api/settings/general`, `GET /api/gateway/devices` → 200; on the local loopback daemon `drain`/`undrain` → 200 (BR-5 canary) |

### Completed BR-1 walk on the paired remote session

- Dock Terminal launcher **absent** (0 matches; local sessions show it). Settings save bar **absent** on `/settings/general`; profile creation **absent** on `/settings/profiles`.
- Dispatching the daemon-owned "Open Terminal" palette command lands in the **truthful loopback-only window**: "This action runs on the machine running CompozyOS — You are connected through remote access, so this action cannot run from this device."
- **Deep-linked settings write (attention toggle) → 403 → the shell renders the truthful loopback-only strip** (T8 / US-002.AC-2 in-product; screenshot `ts2-15`).
- Streams re-verified: single-use tickets (reuse → 401 `gateway_stream_ticket_invalid`), reconnect re-mints, inventory agreement across Web/HTTP/UDS/CLI (5/5/5/5 devices), revocation closing live work (`canceled: 1`, stream closed mid-flight, credential then 401).

### Disclosed residuals (non-blocking, named)

- Notification-delivery toggles on `/settings/attention` still render as live switches on the paired session; toggling refuses 403 into the truthful strip. This is the recorded task_03 residual class (affordance present, server truth holds, gate is a small local edit) — disposition belongs to the loop controller.
- The settings surface's keyboard/screen-reader rows were not re-probed this walk; the 2026-08-07 walk and UI-13 baseline remain the owning evidence for the unchanged plumbing.
- Public tier/Funnel remains out of scope (`RT-gateway-public-ui-consent` owns it).

### Addendum 2 verdicts

- `RT-gateway-paired-device` → **pass** (`fix_status: fixed`, `retest_status: pass`, commit `17304fe8f`): the QR/link pairing flow works end to end through the product UI; single-use, cross-surface agreement, and revocation-closes-live-work all confirmed.
- `RT-gateway-operator-surface-truth` → **pass** (`fix_status: fixed`, `retest_status: pass`, commit `17304fe8f`): live-address presentation, truthful states, BR-1 absences, the 403 → truthful strip backstop, and server-side enforcement all confirmed; named residuals above.

### Addendum 2 final status

- **Issues by user impact (cumulative run):** Blocks-Completion 0 open · Data-Loss 0 · Trust-Damage 0 open (both forwarded-tier defects verified fixed) · Friction 1 open (BUG-20260911-session-rail-docks-on-touch-tier, different charter) · Cosmetic 0
- **Secret disposition:** fresh key deleted after bring-up; zero occurrences by byte-scan; cookie jars deleted; all walk devices revoked; the user should revoke both keys used today at the tailnet.
- **Verdict (addendum 2): the remote-tier surface truth holds** — with the two forwarded-tier defects fixed and verified, E2E-001's paired private-tier journey is confirmed in-product; remaining work for this workflow is the T6 mobile rail fix (other charter) and the recorded non-blocking residuals.

## Repair Re-walk Addendum — 2026-09-11 (BUG-20260911-session-rail-docks-on-touch-tier)

**What changed (repair lane, web composition only):** the task_04 T6 wrapper in
`web/src/systems/os/apps/session/session-window-content.tsx` relied on Chromium
blockifying `display: contents` under `position: absolute` — it never does, so
the media-scoped `max-[760px]:absolute/inset/z/shadow-overlay` classes applied
to a wrapper that generated no box and the rail stayed a docked flex child at
every width (landscape merely looked correct because docking is the >760px
desktop contract). The fix opts the wrapper back into a real display at the
touch tier (`max-[760px]:block`, which wins the cascade over the base
`contents` utility inside the breakpoint) and scopes the overlay shadow to the
rail's open state so a closed rail cannot paint the token's 1px ring.
`SessionSidebar` and all other surfaces untouched; two regression tests added
to the owning suite. Full root cause in the bug's `## Fix`.

**Re-walk (fresh lab `compozy-mobile-surface-truth-repair-20260911-20260911-055549-379543`,
daemon `http://127.0.0.1:34657` serving a fresh `web/dist` build of the fix
via `COMPOZY_WEB_DIST_DIR`, Playwright 1.62.1 Chromium device-emulated
390×844 touch, en-US; teardown `clean: true`):**

- **T6 at 390×844 — PASS.** Rail open: wrapper computes `block`/`absolute`/`z-30`,
  rail overlays x=0..264, transcript keeps full width (x=0, w=390), composer
  editor 332px wide (was 68) with send control intact; computed box-shadow
  carries the `--shadow-overlay` layers. Rail closed: no shadow, transcript
  full width. Evidence `r-1`–`r-3`, `r-6` under
  `docs/qa/evidence/2026-09-11-mobile-surface-truth/`.
- **Landscape 844×390 control — unchanged/correct.** Rail docks as a layout
  column (transcript x=264 w=580, editor w=522), no shadow — the >760px rule
  holding as before. Evidence `r-4`.
- **Desktop 1440×900 spot-check — unchanged.** Docked, no shadow,
  behaviorally identical. Evidence `r-5`.
- Zero browser console errors / page errors across all legs.
- Scoped validation from the repo root: `bunx turbo run typecheck --filter=./web`
  PASS (2/2); `bunx turbo run test --filter=./web` PASS (776 files / 7212
  tests, 0 failures — baseline 7210 + 2 new regression tests);
  `bunx turbo run lint --filter=./web` PASS (0 warnings / 0 errors).

**Environment note:** no provider credential was present in this repair
environment (`ANTHROPIC_API_KEY` absent, unlike the original walk); the
onboarding credential step was completed through the same env-var path bound
to a clearly-labeled placeholder value. No provider spend occurred or was
possible (no provider CLI in the environment); session start still truthfully
fails at the runtime step as recorded in the original walk. The T6 leg needs
only the session window surface, which rendered fully.

**Updated final status:** APP-mobile-touch-tier → `pass` (fix_status `fixed`,
retest_status `pass`); BUG-20260911 → `fixed` with re-walk evidence. The
friction count for this run drops to 0 open items. Overall verdict remains
gated on the unchanged external blocker: the private-tier legs stay
blocked-verify pending the user-provisioned authorized `TS_AUTHKEY` (rows
1–2), and the real-device keyboard-open check remains a human-verification
residual (row 3).

## Addendum 3 — real-device walk (2026-09-11, user's phone)

The paired session was walked on a real phone (Android, Brave, ~390×844) over
the verified private endpoint. Core flows confirmed live: pairing gate → paired
session; surface truth visible (Settings read-only, no save bars); dock, palette
and window chrome rendering at the touch tier.

Real-device findings (both fixed in-cycle, commit `2bd4ca788`):
- **F1 (defect):** the compact DesktopPager rendered a single-desktop position
  pill orphaned at the dock's far edge with ~195px dead zone — dock spacing
  broken at 390px. Fixed: single desktop renders no switcher; multiple desktops
  shrink-wrap.
- **F2 (defect):** window controls identified only via hover tones (no hover on
  touch); the 2-of-3 count is per-design (zoom hidden, close reachable). Fixed:
  rest glyphs for close/minimize at the compact tier.

Artboard evolved: T9 (dock pager) and T10 (window-control identification) added
as real-device deltas. Fixes deployed to the user's runtime; on-device re-walk
pending. T4 keyboard-open on a real device remains a human-verification residual.
