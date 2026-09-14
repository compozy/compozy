# BUG-20260913-marketplace-name-conflict-trail: Marketplace offers Install for an occupied extension name

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-marketplace-acquisition, inspect a plugin from another source
- **Scenarios:** ET-web-marketplace-sources-add
- **Found:** 2026-09-13 · **Report:** docs/qa/reports/2026-09-13-marketplace-catalog.md

## Summary

After installing review-tools from team-plugins, the partner-plugins listing still offers Install.
The lifecycle refuses the competing installation and preserves the first package, but the listing
never explains that the name is occupied, even after a fresh read.

## Reproduction

- **Charter:** CH-marketplace-under-a-minute · **Tour:** Money Tour
- **Environment:** isolated production daemon/Web at e60d78ea1, desktop1440×900.

1. Add two local marketplaces declaring review-tools from different source refs.
2. Install team-plugins/review-tools with explicit trust and package confirmation.
3. Inspect partner-plugins/review-tools, attempt installation, then reload.

**Expected:** Name in use, with the installed origin, while installed identity remains source-qualified.
**Actual:** Install stays available; GET listing omits name_conflict. The installed package is protected.

## Evidence

- ../evidence/2026-09-13-marketplace-catalog/name-conflict-before.json
- Public GET extension review-tools retains origin team-plugins after the rejected competing install.

## Fix

- **Root cause:** The contract already has name_conflict, but catalog projection never assigns it and
  the trail never renders it. Plugin dry-load discards the declared instance name needed for an accurate
  conflict check; the curated publisher already enforces manifest.name == entry_id.
- **Scope:** retain inspected instance name in existing JSON projection, join occupied names separately
  from installed-by-origin, and render the specified state. No schema or public DTO change.
- **Fix commit:** pending
- **Regression test:** existing marketplace source projection, core Marketplace and component suites.

## Verification

Fresh Bruno session on the rebuilt daemon/Web: source refresh and a fresh GET expose the partner listing with Name in use and the team origin; team review-tools remains Installed. Screenshot inspected: ../evidence/2026-09-13-marketplace-catalog/name-conflict-after.png; independent response: name-conflict-after.json. Core race and real plugin-loader integration pass;53component+25controller cases and Web typecheck pass. Fix commit pending final QA checkpoint.
