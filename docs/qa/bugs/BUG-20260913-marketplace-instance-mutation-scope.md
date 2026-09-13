# BUG-20260913-marketplace-instance-mutation-scope: Workspace update omits the selected installation scope

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-marketplace-acquisition, update and remove a workspace installation
- **Scenarios:** ET-web-marketplace-installed-management
- **Found:** 2026-09-13 · **Report:** reports/2026-09-13-marketplace-catalog.md

## Reproduction

Install Brave Search 2.1.0 into team-project, publish 2.1.1 in the isolated catalog, refresh and
click Update in Installed. PUT /api/extensions/brave-search omits workspace/profile and returns 404.
The same request construction affected catalog updates, detail updates and published workspace removal.
Update all also sent one unscoped batch for a potentially mixed global/workspace inventory.

## Root Cause and Repair

Mutation destinations were inferred from development status or omitted instead of using the resolved
installation. A shared extension scope projection now uses the selected extension's workspace/profile.
Catalog updates resolve their joined installed name through the scoped inventory; inherited global
installations remain global. Recovery preserves the complete request. Update all groups installations
by profile/workspace and retains partial results across groups. Removal receives the selected scope.
No endpoint alias, compatibility shim or active-workspace fallback is introduced.

Invariant: every lifecycle mutation targets the selected installation, including inherited globals.
Owners and canonical suites: Marketplace controller and installed page integration; extension lifecycle
hooks and detail state. Focused verification: 104 tests passed, then the extended mixed-scope batch
suite passed 55 tests; TypeScript passed. Fresh production update/removal walk passed.

## Fresh Production Verification

Brave Search updated through Installed from 2.1.0 to 2.1.1 in team-project. Scoped public detail confirms
version, workspace, enabled state and no missing inputs; the declared BRAVE_API_KEY binding remains
present and the row reports Running. The normal Remove dialog requires the exact installed name.
After confirmation, scoped detail returns404, inventory omits Brave, and Vault metadata omits its
exclusive secret. Sentry extension OAuth credentials remain present/authenticated/ready; the same-name
manual sentry remains independently ready. No baseline extension row was removed.
Evidence: workspace-update-after.json, brave-search-secrets-before-removal.json,
workspace-remove-confirm.png, inventory-after-workspace-removal.json, vault-{before,after}-removal.json,
sentry-{manual,extension-sentry}-after-brave-removal.json under the report evidence directory.
