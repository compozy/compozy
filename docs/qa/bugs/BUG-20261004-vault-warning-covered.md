# BUG-20261004-vault-warning-covered: Replace it covers part of the Vault overwrite warning

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-keep-secrets-contained, confirm replacement of an existing secret
- **Scenarios:** ET-web-vault-overwrite-confirmation; MS-web-entity-modal-shell
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction and evidence

CH-untested-061-keep-secrets-contained-dora, Garbage Tour, real production Web at 1512 × 862.
Save `automation/editorial/notes-token`, filter the listing to hide it, then choose New secret
and enter the same name with a new synthetic value. The overwrite warning appears and Save secret
correctly requires Replace it, but that switch and label cover the end of the warning's first line.
The consequence should remain fully readable before consent.

Evidence under docs/qa/evidence/2026-10-02-untested/:
vault-name-recovery-dora-overwrite-warning.png and
vault-name-recovery-dora-confirmation-retracted.png. Both screenshots were inspected. The
40-frame vault-name-recovery-dora recording is closed and the disposable binding was deleted.
Registry searches for overlap, obscured content, overwrite and Replace it found no existing owner.

## Diagnosis and repair boundary

Vault composes the wide confirmation control in `AlertAction`, whose existing contract places
compact controls absolutely at the upper trailing corner. The existing `AlertActions` primitive
puts actions in a wrapping row in normal flow. Use that exported primitive; change no shared
styles, confirmation state, validation, persistence, or other alert consumers.

The invariant is readable consequence text and a separate operable confirmation control at
desktop and narrow widths. The owning evidence is a real browser replay; a JSDOM assertion of
CSS classes would not prove layout and is not added. The existing use-vault-page suite retains
the separate behavioral invariant for overwrite consent and name changes.

## Repair and replay

- **Fix commit:** baec8d019cd6113ce7db0c2811118724eebd94a7.
- **Regression evidence:** vault-warning-dora-layout-consent.json and its desktop, narrow, small,
  and reflow screenshots under docs/qa/evidence/2026-10-02-untested/. Each was inspected.
- The real warning has separate description/action rectangles at 1512 × 862, 720 × 900,
  375 × 900 and 756 × 431. Text and controls have no clipping or overlap. The final case is
  half-size viewport reflow, not a claim of native browser 200% zoom. RTL, native browser zoom,
  and translated content are not verified by this en-US persona walk.
- Replace it enables saving, changing away and back retracts consent, and Cancel leaves exact
  metadata unchanged. The disposable binding is deleted and the final CLI list is empty.
- The 18-frame recording is closed at
  /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/vault-warning-dora.
  vault-warning-build-identity.json pins the exact served bundle and source hashes.

This fixes the Vault composition using the existing action-row primitive; it does not change
shared Alert behavior or claim a general responsive/RTL audit.

## Verified delivery — 2026-10-04

The original-persona replay documented above passes. The final affected gate exits 0, including
Go race suites, 6,951 Web tests, UI checks, generation, lint and types. Commit baec8d019cd6113ce7db0c2811118724eebd94a7
contains exactly the tested tree 1db800a634d9066def935863fe3e62a6013c7df9.
Receipts: docs/qa/evidence/2026-10-02-untested/settings-vault-final-delivery-gate.json and
docs/qa/evidence/2026-10-02-untested/settings-vault-final-commit-identity.json. Earlier pending
checkpoint wording records history; this section closes this defect, not the overall QA scope.
