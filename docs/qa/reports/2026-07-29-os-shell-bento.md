# QA Run Report — 2026-07-29 — os-shell-bento

- **Scope:** Landing-page OS Shell bento update: new 4:3 illustration and revised OS Shell claim.
- **Cadence tier:** sanity
- **Build:** working tree · **Environment:** local site at `http://127.0.0.1:3000`, production-like Next.js rendering; desktop and mobile capture only.
- **Started:** 2026-07-29T00:00:00-03:00 · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Cora | Casual User | desktop + mobile / wifi-fast / en-US | CH-compozy-landing-canary |

## Flows in Scope

- `J-evaluate-compozy-beta` — A first-time reader judges the integrated OS claim from the local landing (`../journeys/J-evaluate-compozy-beta.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-compozy-landing-canary | J-evaluate-compozy-beta / REL-os-landing-proof | Cora | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-compozy-landing-canary — Cora

- **Ran:** local browser capture at 1440×900 and 390×844 (box respected: yes).
- **Findings:** The `OS SHELL` card makes the managed-window model legible without relying on in-image text. At desktop width, all five windows and their central control rail remain visible; at mobile width, the active-window cluster and control rail remain visible after responsive crop.
- **Bugs filed/updated:** None.
- **Scenarios settled:** `REL-os-landing-proof → pass`.
- **Paper cuts:** None observed.
- **Surprises:** The Next.js optimizer retained the previous art for an unchanged source URL; versioning the public image source resolved the stale asset without adding a control or changing runtime behavior.
- **Suggested next charter:** Re-walk the full landing only when its hero, comparison, installation, or CTA contract changes.

## What Was Fixed

None.

## Paper Cuts

None observed.

## Runtime Errors Observed

None observed.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A versioned public image source is required after changing a Next.js image asset; the local optimizer otherwise retained the prior source image under the old URL.

## Final Status

Pending session walk and full automated verification.
