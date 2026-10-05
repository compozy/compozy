# BUG-20261004-vault-help-name-generic: Vault naming help does not identify its field

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, identify help beside a required secret name
- **Scenarios:** MS-web-modal-help-tips; MS-web-entity-modal-shell
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction and evidence

CH-untested-041-administer-runtime-settings-dora, Back-Button Tour. From Tasks open Vault,
then New secret. The Name field's new help opens correctly, but its accessible name is
`More information`, rather than identifying Name as required by the shared help contract.
The field itself remains `Name required`, and the help button is correctly outside its label.
This was found in the uncommitted name-recovery repair, before delivery.

Evidence: docs/qa/evidence/2026-10-02-untested/modal-last-entries-dora-vault-help-ended.json
and modal-last-entries-dora-vault-help-name.png. The screenshot is inspected; the 26-frame
modal-last-entries-dora recording is closed. Registry searches found no owner for this symptom.

## Diagnosis and repair

Settings rows derive their help name only from plain string labels. Vault's Name includes the
existing RequiredMark, so the fallback loses field identity. Add an explicit optional helpLabel
to the existing row contract and supply `About name` in Vault. Preserve the visible required
marker, input association, separate help button, validation and write-only behavior. Do not
infer labels by traversing arbitrary React children or change the shared HelpTip lifecycle.

- **Fix commit:** baec8d019cd6113ce7db0c2811118724eebd94a7.
- **Regression test:** existing settings-field-row.test.tsx, decorated-label help naming in both
  row presentations. It asserts the specific button name, separate label ownership, and exact
  field name. settings-help-decorated-label-red.json fails on the original generic name.
- The first post-fix run reaches the field-name assertion but exposes missing literal spacing
  in the new native-input fixture under JSDOM (`Namerequired`). The real Chrome field already
  reads `Name required`. The fixture gains a literal separator; the exact assertion is retained.
- **Retest:** the fresh 20-frame settings-vault-final-ready-dora recording passes the contextual
  help checks: About name is separate from Name required, keyboard focus has a 2px ring, and the
  target measures 24 × 24 on desktop and 44 × 44 at 720px. Keyboard and touch expose the guidance;
  Escape retains the draft. Cancel and an independent empty Vault list confirm no saved value.
  The contextual-help receipt ends on a later Close Vault lookup during viewport transition;
  the following help-complete-state and restored-ended receipts finish the session. This lookup
  is not a help failure. All screenshot checkpoints are inspected. The final focused suite passes
  all 12 tests; Web typecheck/build pass. Gate and fix-commit recording remain outstanding.

## Verified delivery — 2026-10-04

The original-persona replay documented above passes. The final affected gate exits 0, including
Go race suites, 6,951 Web tests, UI checks, generation, lint and types. Commit baec8d019cd6113ce7db0c2811118724eebd94a7
contains exactly the tested tree 1db800a634d9066def935863fe3e62a6013c7df9.
Receipts: docs/qa/evidence/2026-10-02-untested/settings-vault-final-delivery-gate.json and
docs/qa/evidence/2026-10-02-untested/settings-vault-final-commit-identity.json. Earlier pending
checkpoint wording records history; this section closes this defect, not the overall QA scope.
