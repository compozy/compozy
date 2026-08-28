# QA Run Report — 2026-08-27 — Runtime and UI regressions

- **Scope:** Working-tree fixes for desktop runtime coherence, strict-CSP font loading, Settings scroll ownership, cold model-catalog latency, and default-profile agent launch
- **Cadence tier:** targeted
- **Build:** `459100d95` + working tree · **Environment:** isolated local macOS lab; daemon-served production web bundle and packaged Electron shell
- **Started:** 2026-08-27T15:53:12Z · **Completed:** 2026-08-27T16:36:00Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Runtime administrator | desktop / wifi-fast / en-US | CH-runtime-ui-regression-settings-scroll, CH-runtime-ui-regression-agent-skills, CH-runtime-ui-regression-fonts |
| Sol | Accessibility-Reliant | desktop / wifi-fast / en-US | CH-runtime-ui-regression-model-catalog |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-runtime-ui-regression-update-coherence |

## Flows in Scope

- `J-administer-runtime-settings` — inspect Settings without a second document scroll (`../journeys/J-administer-runtime-settings.md`)
- `J-17` — create a session with a newly discovered agent and open the cold runtime selector (`../journeys/J-17-session-create-unified-selector.md`)
- `J-operate-desktop-shell` — load production font assets under the strict CSP (`../journeys/J-operate-desktop-shell.md`)
- `J-desktop-attach-daily` — reconcile an updated app with its stale app-owned runtime (`../journeys/J-desktop-attach-daily.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-runtime-ui-regression-settings-scroll | J-administer-runtime-settings / MS-settings-single-scroll-owner | Dora | Back-Button Tour | Pass | | |
| 2 | CH-runtime-ui-regression-model-catalog | J-17 / RT-model-catalog-cold-open | Sol | Feature Tour | Pass | | |
| 3 | CH-runtime-ui-regression-agent-skills | J-17 / RT-agent-hot-discovery-skill-isolation | Dora | Feature Tour | Pass | | |
| 4 | CH-runtime-ui-regression-fonts | J-operate-desktop-shell / ET-web-font-assets-strict-csp | Dora | Feature Tour | Pass | | |
| 5 | CH-runtime-ui-regression-update-coherence | J-desktop-attach-daily / APP-desktop-runtime-bundle-coherence | Bruno | Interrupt Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

1. General reached its exact inner bottom at 1244px; repeated downward input left the document at zero and produced no blank tail.
2. The first onboarding selector opened usable persisted rows in about 420ms while live probing stayed outside the request path.
3. The open app discovered `marketing` without reload. A second agent with valid `brief` and invalid `signup` created one active session with HTTP 201; only `signup` was omitted and diagnosed.
4. Same-origin WOFF2 files loaded under the unchanged strict CSP in the daemon-served bundle and packaged Electron shell.
5. The packaged N+1 launch stopped and replaced the healthy app-owned N runtime, matched the new bundle/provenance, and preserved the home sentinel.

## What Was Fixed

- Removed the second Settings scroll owner.
- Moved live model discovery to daemon-owned background warmup.
- Unified default-profile agent listing, watching, and session resolution.
- Isolated each invalid agent-local skill while retaining valid siblings and allowing session creation.
- Emitted font assets instead of CSP-blocked data URLs.
- Reconciled healthy stale app-owned runtimes with the app bundle before attach.

## Paper Cuts

- The embedded pinned web-assets module still represented the pre-fix bundle during the first baseline reproduction. Candidate verification used the current production `web/dist`; the release workflow publishes and pins those assets before assembling the release.

## Runtime Errors Observed

- Baseline reproduction logged the reported `font-src` violation from the previously pinned asset bundle. The candidate daemon-served bundle and packaged app had no font CSP violation or browser error.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

None identified yet.

## Learnings

- Invalid skills are a per-file diagnostic boundary, not an agent or session availability boundary.
- Desktop ownership is strong enough to repair app/runtime skew safely without deleting runtime home state.

## Final Status

- **Exit gate (full automated suite):** PASS — `make gate`, fingerprint `6160882da6fc6c2fe098e0622551e765ea726299`; current `go-lint`, `go-test`, `js-desktop`, and `js-web` records under `.cache/gate/logs/`
- **Issues by user impact:** 0 blocking, 0 non-blocking
- **Coverage:** 5/5 scenarios walked and passed
- **Verdict:** PASS locally. Exact-head PR CI remains outside this uncommitted working-tree run.
