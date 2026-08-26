# BUG-20260825-custom-source-stuck-pending: An active custom source stays pending in Settings

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-absorb-skills-from-other-tools, add a custom source in Settings
- **Scenarios:** ET-web-skill-sources-settings
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

Dora adds a valid custom skill folder and the daemon applies it, but Settings keeps the row in a
pending state instead of showing the measured skill count. A fresh API read proves the source is
active, so the screen contradicts persisted runtime truth.

## Reproduction

- **Charter:** CH-skill-sources-settings-web · **Tour:** Garbage Tour
- **Environment:** desktop / 1280×900 / wifi-fast / en-US, daemon-served production bundle

1. Open Settings > Skills.
2. Add a custom folder whose path resolves through the macOS `/var` to `/private/var` alias.
3. Save, then wait for source diagnostics and refresh the page.

**Expected:** the configured path remains visible and receives the measured skill count.
**Actual:** the API returns the canonical measured root, while the screen creates a second pending
row for the configured spelling.

## Evidence

- /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/settings-http-after.json
- /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/browser-e2e

## Fix

- **Root cause:** source measurement canonicalizes filesystem paths, but the Settings projection
  joined measured custom roots back to configuration with byte-for-byte path equality. The
  configured `/var/…` path and measured `/private/var/…` path therefore became two rows.
- **Fix commit:** `df739b0`
- **Regression test:** `internal/settings/skill_diagnostics_test.go`,
  `TestSkillsSectionDiagnostics/Should preserve configured custom paths beside canonical root measurements`.

## Verification

- **Retested:** 2026-08-25, same persona/journey · **Report:** docs/qa/reports/2026-08-25-skill-sources.md
- **Result:** E2E-008 passes after a fresh daemon-served run; the row shows one measured skill and
  remains removable by its configured path.
