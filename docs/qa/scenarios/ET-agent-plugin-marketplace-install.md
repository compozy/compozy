---
id: ET-agent-plugin-marketplace-install
area: ET
title: Install an Agent Plugins catalog entry from Marketplace
persona: Bruno
journey: J-marketplace-acquisition
expected: A catalog entry marked `format: agent-plugin` shows a neutral Agent Plugin badge on the card and detail view, follows the normal trust and install flow, lands on extension management with format and skipped diagnostics visible, and still relies on acquired-package detection when catalog metadata is absent or stale.
entry_points: Web /marketplace and entry detail; Web extension trust/install dialog and /settings/extensions; compozy marketplace search; GET /api/marketplace over HTTP and UDS; POST /api/extensions; curated catalog feed
qa_status: pass
bug_ids: BUG-20261107-derived-secret-env-enable-fails; BUG-20261107-enablement-route-500; BUG-20261107-set-up-waits-on-slow-mcp-read
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /tmp/mpc-shots/final (add-ready.png, set-up.png, turn-on.png, enable-failed-retry.png)
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-catalog-navigation; ET-web-marketplace-detail-redesign; ET-web-marketplace-installed-management
---

QA impact 2026-08-16: Marketplace can display the portable format and the extension detail surface can
show ingest skips. Task 08 must drive the browser flow and compare it with structured catalog and
installed payloads; the badge must never override runtime detection.

QA 2026-08-16: card, detail, neutral badge, trust dialog, installed management, and skipped inventory
were walked in the real browser. The fixture catalog's synthetic GitHub release URL returned 404 at
the final install mutation, so acquisition itself remains `blocked-verify`; the same bytes were then
installed through the public CLI to verify the installed Web state without weakening HTTPS/SSRF rules.

QA impact 2026-09-13 (task_06; final live/visual owner task_10 VC-05): install client
layouts from local paths and sources through the current extension surfaces. Verify root
manifest precedence, recorded layout, unchanged source/package bytes, and only authored
resources. Open Design contributes one MCP and zero packaged skills; loop-engineering
contributes seven skills and no MCP. An explicit package with commands/agents/hooks must
report `client_component_ignored` with zero loaded hooks. Verify update, removal, dev reload,
trust and scoped resource delivery. Focused Go lifecycle evidence does not close this live row;
previous browser evidence above predates the client adapter and current Marketplace routes.

Task03 input step (final tasks09/10): for a plugin that declares inputs, confirm unverified trust
explicitly, review the acquired manifest and complete the same typed fields used by curated
extensions. The request retains the approved digest and allow_unverified; input edits do not
reacquire a preview. A source change invalidates confirmation. An update requiring new values
uses the candidate input_definitions response, keeping the selected instance and prior inputs.
After restarting the daemon, confirm that configured MCPs retain their URL and boolean inputs.
A second profile without those inputs must still report missing configuration and publish no MCPs
from that package; it must not prevent installation or publication in the configured profile.
Do not invent input fields for a package that declares none.

Task07 acquisition (final live/visual owner task10): configure a folder source and refresh; inspect
its digest, contents, and unverified decision in Web and HTTP/UDS. Install through source=marketplace
with the selected expected_digest and explicit consent. Refresh changed bytes after confirmation and
verify409 extension_source_changed with no mutation. With a cached blob, change or hide the folder:
installation still uses approved bytes. Without the blob and with the folder inaccessible, verify503
source_unreachable. Restore equal bytes and retry. Register the same source under another name and
verify both rows join the one instance; another origin claiming its name must conflict. Update changed
bytes with an unchanged version and verify new provenance plus preserved inputs/attachments/rollback.
Focused daemon integration and controller tests are receipts, not completion of this live browser row.

CI repair impact 2026-09-13: the browser suite installs both standard and .claude-plugin packages through trust and install-summary confirmation. Read the resulting instance using its returned workspace/profile; an unscoped 404 must not be treated as installation failure. The current layout value is claude-plugin. Unsupported component diagnostics remain visible on the standard package.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.


## QA impact and focused evidence 2026-11-02 — responsiveness audit

Catalog availability now reuses a bounded, expected-digest-bound verification result only for unchanged securely opened package bytes. Installation still opens and hashes the approved bytes. Catalog availability and permission to install remain distinct; replacement, corruption, missing files and digest changes must not inherit an earlier successful check.

Focused verification passed under `-race`: `TestPackageCache` (`internal/marketplace/pluginsource/cache_test.go`, 2.281 s package receipt in `.cache/performance/extension-marketplace/focused-race.txt`) and `TestResolveAndAcquire` (`internal/marketplace/pluginsource/acquisition_test.go`, 1.457 s package receipt in `integration-race.txt`). `TestCatalogServiceDetailAndStatus` (`internal/marketplace/service_test.go`, 5.964 s in `marketplace-race-final.txt`) verifies truthful missing-package blockers and recovery. The mixed logs retain unrelated temporary compile failures; only their successful package rows are cited here.

Existing root CI mappings, not rerun by this worker: `TestExtensionMarketplacePortableInstallParityIntegration` (`internal/cli/extension_marketplace_integration_test.go`); `TestDaemonE2EExtensionDistributionAcrossIsolatedHomes/Should install and update pinned plugins through public transports [IT-015 IT-018 IT-019]` (`internal/daemon/daemon_extension_distribution_e2e_integration_test.go`). Browser ownership remains `operator installs a badged catalog entry and reads its ingested and skipped components`, `operator installs a curated client-layout package with its layout provenance`, and `E2E-004: Add marketplace installs client-layout plugins only after approving current bytes` (`web/e2e/__tests__/marketplace.spec.ts`).

No new trust/install browser journey ran; the skipped/pending historical verdict stays unchanged. Full bounded commands, integrity cases and limits: [extension/marketplace audit](../reports/2026-11-01-performance/extension-marketplace.md).

## QA impact 2026-11-07 — marketplace plugin catalog (spec `marketplace-plugin-catalog`, S8)

Install from Marketplace is now one-step **Add** = install + enable (US-005–US-007, ADR-003;
`_uiux.md` S3, S5, S6). Every trust, conflict, gateway and required-input gate still applies.

1. **Add inline:** on a trusted entry with no gate to decide, Add → "Adding…" → toast
   "{name} added" with no dialog; the row ends **Ready** only after the installed inventory confirms
   no missing input and no server waiting for sign-in. `compozy extension status <name> -o json`
   shows it enabled.
2. **Review dialog only on a gate:** an unverified-source or required-input entry opens the review
   dialog; its primary button reads **Add** and confirming installs and enables.
3. **Set up:** an entry with a server waiting for sign-in or a missing input reads **Set up**;
   activating it starts the sign-in or opens detail with `?setup=1` focused on the first missing
   input. `compozy__mcp_auth_status` agrees.
4. **Turn on:** disable the extension from Installed; the row reads **Turn on** and turning it on
   restores Ready/Set up.
5. **Enable failure:** when enabling fails after install (for example a package whose enable is
   refused), the row reads "Couldn't turn on" with **Retry** ("Retry turning on {name}"), the
   toast reads "Added {name} but couldn't turn it on", and the package stays installed.
   CLI parity: `compozy extension install <ref> --enable` exits 0 with `enable_error` and the retry
   command. Record when the failure cannot be reproduced on real data.
6. Detail shows the "From {source}" attribution line.

Capture an Add that ends Ready, a Set up state, Turn on, and the Retry state when reproducible.

### Walk result 2026-11-07 (S8 integrated lab, `feat/mpc-s8` with S1–S7 + S9)

Isolated lab (own home, ports 49760/49761, worktree web dev server), default sources plus a local `lab-plugins` folder source; cold refresh of the default sources took ~65.7 s (compozy-catalog 18, claude-plugins-official 316, compozy-plugins 333). Screenshots and CLI captures are lab-local in `/tmp/mpc-shots/s8/` (not committed).

- PASS — Add → Ready: `lab-plugins/notes-helper` (unverified) asks for consent once ("Install anyway"),
  then the row reads Ready with no second dialog.
- PASS — Review dialog only on a gate: `lab-plugins/demo-api` with required values opens "Add demo-api"
  listing DEMO_API_KEY, DEMO_HOST and DEMO_MODE · optional; its primary button reads Add.
- PASS — inline Add → Set up: official `compozy-catalog/linear` added without a dialog and reads Set up
  (no sign-in performed).
- PASS — Turn on: after `compozy extension disable notes-helper`, the row reads Turn on; pressing it
  returns to Ready.
- PASS (state) / **FAIL** (cause and recovery) — `demo-api` reads "Couldn't turn on" with Retry and stays
  installed, but enabling can never succeed (BUG-20261107-derived-secret-env-enable-fails) and Retry
  shows "Internal Server Error" (BUG-20261107-enablement-route-500). The "Added {name} but couldn't
  turn it on" toast was not captured within the 6 s wait.
- Note: the unverified trust dialog's primary button reads "Install anyway", not "Add".

### Re-walk 2026-11-07 (integration 30fcdfee8, fresh isolated lab)

Cold refresh of the default sources took ~87.2 s (compozy-catalog 18 from a generator-regenerated temp feed, claude-plugins-official 316, compozy-plugins 333). Evidence is lab-local in `/tmp/mpc-shots/final/` (not committed).

- PASS — Add → Ready: notes-helper, one "Install anyway" consent, toast "notes-helper added".
- PASS — inline Add → Set up: Linear with toast "Linear added · Needs authorization before it can run".
  The button is disabled with a spinner for 10–20 s until the live server read returns
  (BUG-20261107-set-up-waits-on-slow-mcp-read).
- PASS — Turn on returns to Ready.
- PASS — enable failure (remote-probe with host `bad host`): the row reads Couldn't turn on with
  Retry, the toast reads "Added remote-probe but couldn't turn it on" plus the daemon cause, and Retry
  shows the cause and `extension_enable_failed` (no Internal Server Error). Polish: one Retry press shows
  two toasts with the same message.
