---
id: ET-web-vault-opendesign-listing
area: ET
title: Vault listing matches OpenDesign inspect model
persona: Bruno
journey: J-marketplace-acquisition
expected: `/vault` shows ListingPage + PageHead + topbar Refresh/New secret, URL-synced prefix/namespace/view, Rows/Cards toggle, one security note ("Values are encrypted. You can't view a secret after you save it."), interactive rows/cards titled by the friendly secret name (full ref in the hover title) that open a detail sheet with Updated/Created facts, masked value with a Saved/Missing word, Replace value → Save, copy ref, and a foot delete confirm. Save/delete confirm with a toast ("Saved <name>" / "Deleted <name>"). Create remains a write-only SettingsEditorDialog. No plaintext secret values appear.
entry_points: Sidebar Vault; /vault; vault secret sheet
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-web-page-content-gutter; ET-web-route-chrome-topbar
---

Added by vault OpenDesign redesign. Flag only — retest in the next QA cycle.

QA impact 2026-07-18: rejecting or unavailable Clipboard API writes now produce a recoverable
copy error without an unhandled promise rejection.

QA impact 2026-09-28 (ui-normie-pass): the copy error is now the shared copy button's failed state
plus an error toast; the sheet's CLI footer is gone; non-session deletes ask the user to type the
secret's friendly name instead of the full ref.

QA impact 2026-07-18: filtered Vault deep links now preload the exact namespace and prefix from the
URL instead of warming the unfiltered cache before the route mounts.

QA impact 2026-07-18: Cards view now exposes the same secret-delete confirmation entry point as
Rows view while retaining inspect selection, metadata, and redaction behavior.

QA impact 2026-07-18: switching namespace clears a prefix owned by another namespace, including
validated deep links, and delete remains unavailable until an in-flight replacement settles so a
confirmed delete cannot be recreated by the earlier write.

qa-impact: 2026-09-30 shell rail Q2. Type-to-confirm dialogs (vault delete, and every `ConfirmDialog` with `confirmTyping`) open with focus on the confirmation input instead of Cancel; plain confirms still open on Cancel. Walk: Delete from the detail sheet, type the name straight away.
