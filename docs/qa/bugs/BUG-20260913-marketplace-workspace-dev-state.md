# BUG-20260913-marketplace-workspace-dev-state: Published workspace installation is shown as a development overlay

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-marketplace-acquisition, update and manage a required-input workspace installation
- **Scenarios:** ET-web-marketplace-installed-management
- **Found:** 2026-09-13 · **Report:** reports/2026-09-13-marketplace-catalog.md

## Reproduction

Install the curated Brave Search package into team-project with its required secret, then publish a
new lab catalog version and refresh. Installed lists the new version but disables Enable, omits the
individual Update action, and offers Unlink dev overlay. Public extension detail reports dev:true even
though the package was acquired from the catalog without a development link.

## Root Cause and Repair

DescribeExtensionForProfile infers development from any nonempty WorkspaceID. Published workspace
attachments now carry that field too; their scope must not imply a development link. DevLink is the
explicit runtime owner. Repair is deployed and the fresh update/removal walk passed.
Invariant: published attachments remain normal installations, while explicit DevLink stays development.
Owner and canonical suite: extension snapshot projection, TestDescribeExtension in describe_test.go.

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
