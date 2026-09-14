# BUG-20260913-marketplace-version-digest-conflict: Refreshed plugin approval returns 404 instead of a digest conflict

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-marketplace-acquisition, require fresh approval after a catalog revision
- **Scenarios:** ET-web-marketplace-sources-add
- **Found:** 2026-09-13 · **Report:** docs/qa/reports/2026-09-13-marketplace-catalog.md

## Reproduction and Contract

Open a plugin install confirmation, change its source package, refresh the catalog, then submit the
old confirmation. The request includes both the old listed version and digest. E2E-004 received404
rather than extension_source_changed409, preventing the UI's fresh-consent recovery.
An unchanged listing with its approved immutable cached bytes remains installable by task07's contract;
changing only the upstream folder does not invalidate that approved blob. The browser fixture now
publishes the new catalog revision before exercising the stale-approval fence.

## Repair and Verification

Resolve the current source-qualified plugin first, check the approved digest, then enforce the requested
version. This preserves explicit version validation and returns the actionable conflict before a stale
version can hide it. The canonical daemon distribution integration now includes the listed version in
its stale request. HTTP/UDS pinned acquisition/update integration passed with race detection in17.789s.
Fresh E2E-004 browser walk passed (4.6s test,8.1s lane): old approval returns409, scoped inventory remains unchanged, new consent installs the current package. No fallback install or automatic consent is introduced.
