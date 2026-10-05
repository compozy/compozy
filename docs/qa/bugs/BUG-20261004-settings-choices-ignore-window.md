# BUG-20261004-settings-choices-ignore-window: Permission choices ignore the compact Settings layout

- **Status:** verified
- **Impact (user-side):** Cosmetic
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, read permission choices in a compact window
- **Scenarios:** MS-web-settings-takeover-redesign
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction and evidence

CH-untested-041-administer-runtime-settings-dora, Back-Button Tour. Open Settings / General and
reduce the viewport to 864 × 900. Navigation becomes a horizontal strip, but the three permission
cards remain side by side. The named General prototype stacks them in this compact state.
Text remains readable at the observed width and no setting changes; this is a visual-contract
divergence, not a failure to apply permissions.

Reference: docs/design/opendesign/_done/settings/settings-general.html,
git object c2fccb10c695cc3319baa376d7668b60688da407. Evidence under
docs/qa/evidence/2026-10-02-untested/qa/visual-contract/settings-takeover/VC-S2-before/ includes the
dimension-matched reference/implementation pair, side-by-side, diff, comparison.json and review.
The original implementation is preserved with the pre-fix evidence. The four-frame
settings-visual-entry-dora recording is closed; the persona made no settings mutation.
Registry searches found no existing owner for this responsive mismatch.

## Diagnosis and repair

SettingsChoiceGroup uses the screen-level sm breakpoint while SettingsWindow uses its own
container and the established --container-settings-takeover token. Reuse the same
@min-settings-takeover variant for the three-column choice layout. No new breakpoint, stylesheet,
state, permission policy, setting key, or save behavior is introduced.

- **Fix commit:** baec8d019cd6113ce7db0c2811118724eebd94a7.
- **Regression evidence:** real before/after viewport and window-container layout captures.
  A JSDOM class assertion cannot prove reflow and is not added. The existing SettingsChoiceGroup
  suite continues to own radio keyboard selection, including RTL direction.
- **Retest:** the fresh settings-vault-final-ready-dora recording measures three columns at
  1440 × 900 and three stacked cards at 864 × 900, retaining the selected permission. Final
  VC-S1 and VC-S2 pairs, side-by-side images and diffs are inspected; both reviews have zero
  blocking divergences and both bundle validators pass. The pre-fix VC-S2 failure remains in
  VC-S2-before. These pairs cover clean General only. The existing keyboard/RTL choice tests,
  Web typecheck and build pass. Gate and fix-commit recording remain outstanding.

## Verified delivery — 2026-10-04

The original-persona replay documented above passes. The final affected gate exits 0, including
Go race suites, 6,951 Web tests, UI checks, generation, lint and types. Commit baec8d019cd6113ce7db0c2811118724eebd94a7
contains exactly the tested tree 1db800a634d9066def935863fe3e62a6013c7df9.
Receipts: docs/qa/evidence/2026-10-02-untested/settings-vault-final-delivery-gate.json and
docs/qa/evidence/2026-10-02-untested/settings-vault-final-commit-identity.json. Earlier pending
checkpoint wording records history; this section closes this defect, not the overall QA scope.
