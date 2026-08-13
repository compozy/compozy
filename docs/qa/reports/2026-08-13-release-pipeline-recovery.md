# QA Run Report — 2026-08-13 — release-pipeline-recovery

- **Scope:** Recover beta.15 without republishing immutable npm versions, repair the signed beta.13 desktop feed, and prove the next automatic beta publishes GitHub assets before installable npm packages.
- **Cadence tier:** targeted
- **Build:** working tree · **Environment:** hosted GitHub Actions runners, public GitHub/npm/R2 distribution, isolated local QA lab
- **Started:** 2026-08-13T18:20:39-03:00 · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Technical evaluator | macOS and Linux hosted runners / public network / en-US | CH-beta-install-channels, CH-published-npm-channel-readiness |
| Lea | New user | clean macOS and Linux package homes / public network / en-US | CH-desktop-first-run-macos, CH-desktop-first-run-linux |

## Flows in Scope

- `J-evaluate-compozy-beta` — install one published beta through its public channels (`../journeys/J-evaluate-compozy-beta.md`)
- `J-publish-compozy-beta` — publish one beta and wait for public package policy to converge (`../journeys/J-publish-compozy-beta.md`)
- `J-desktop-first-run` — install a desktop package and provision from the signed live runtime feed (`../journeys/J-desktop-first-run.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-beta-install-channels | J-evaluate-compozy-beta / REL-beta-install-paths (beta.15 recovery) | Dora | Feature Tour | Pending | | |
| 2 | CH-desktop-first-run-macos | J-desktop-first-run / APP-install-first-run-provision | Lea | Feature Tour | Pending | | |
| 3 | CH-desktop-first-run-linux | J-desktop-first-run / APP-install-first-run-provision | Lea | Feature Tour | Pending | | |
| 4 | CH-beta-install-channels | J-evaluate-compozy-beta / REL-beta-install-paths (next automatic beta) | Dora | Feature Tour | Pending | | |
| 5 | CH-published-npm-channel-readiness | J-publish-compozy-beta / REL-published-npm-channel-readiness | Dora | Network Tour | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Pending hosted publication and install walks.

## What Was Fixed

Pending final commit id and hosted replay.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

- beta.15 npm postinstall returned HTTP 404 because the matching GitHub Release asset was not public.
- beta.13 packaged desktop startup rejected the signed runtime manifest because its bytes were not canonical JSON.

## Human Verifications Needed

None currently. macOS and Linux package behavior is exercised by clean hosted runners.

## Decisions for a Human

None.

## Learnings

- Registry publication must be treated as irreversible input when allocating the next beta, even when an earlier release job failed.

## Final Status

- **Exit gate (full automated suite):** pending
- **Issues by user impact:** Blocks-Completion 2 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 0/3 journeys walked
- **Verdict:** not ready — hosted recovery, public installs, npm readiness, and the full gate are pending.
