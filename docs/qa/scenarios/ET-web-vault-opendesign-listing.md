---
id: ET-web-vault-opendesign-listing
area: ET
title: Vault listing matches OpenDesign inspect model
persona: Bruno
journey: J-marketplace-acquisition
expected: `/vault` shows ListingPage + PageHead + topbar Refresh/New secret, URL-synced prefix/namespace/view, Rows/Cards toggle, security note, interactive rows/cards that open a detail sheet with redacted tiles, masked value, rotate Store, copy ref, CLI foot, and delete confirm. Create remains write-only SettingsEditorDialog. No plaintext secret values appear.
entry_points: Sidebar Vault; /vault; vault secret sheet
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-page-content-gutter; ET-web-route-chrome-topbar
---

Added by vault OpenDesign redesign. Flag only — retest in the next QA cycle.

QA impact 2026-07-18: rejecting or unavailable Clipboard API writes now produce a recoverable
inline copy error without an unhandled promise rejection.

QA impact 2026-07-18: filtered Vault deep links now preload the exact namespace and prefix from the
URL instead of warming the unfiltered cache before the route mounts.

QA impact 2026-07-18: Cards view now exposes the same secret-delete confirmation entry point as
Rows view while retaining inspect selection, metadata, and redaction behavior.

QA impact 2026-07-18: switching namespace clears a prefix owned by another namespace, including
validated deep links, and delete remains unavailable until an in-flight replacement settles so a
confirmed delete cannot be recreated by the earlier write.
