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
| 1 | CH-gateway-paired-operator-surface | J-expose-and-pair-gateway / RT-gateway-paired-device | Iris | Truthful Surface Tour | Blocked (needs human verify) | | |
| 2 | CH-gateway-paired-operator-surface | J-expose-and-pair-gateway / RT-gateway-operator-surface-truth | Iris | Truthful Surface Tour | Blocked (needs human verify) | | |
| 3 | CH-mobile-shell-touch-tier | J-operate-desktop-shell / APP-mobile-touch-tier | Marina | Touch Tier Tour | Fixed | BUG-20260911-session-rail-docks-on-touch-tier | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

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

- [ ] Provision an authorized `TS_AUTHKEY` (user-provisioned external prerequisite), enable the tailscale provider for the private tier, and verify the private listener answers on its real address; then walk: paired second device admission, absent terminal/write affordances on the private tier, deep-linked write → 403 `loopback_mutation_required` → truthful loopback-only strip naming the host, fresh single-use stream tickets with reconnection re-mint, live revocation closing streams, and public consent/ingress legs (rows 1–2).
- [ ] On a real phone (or device-emulated mobile browser with virtual keyboard), verify the keyboard-open `interactive-widget=resizes-content` contract: composer and palette input stay above the keyboard (row 3 residual).
- [ ] After BUG-20260911 is fixed: re-walk the T6 rail overlay at 390×844, plus the scroll-pill and log-body scroll-well residuals that were not reachable this run (row 3).

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
