# BUG-20261003-profile-emoji-keyboard-unreachable: Arrow navigation stays on the first emoji result

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Sol
- **Journey Step:** J-operate-profiles, choose an emoji identity
- **Scenarios:** ET-profile-switcher-restore; ET-profile-web-settings-lifecycle-dialogs
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

In CH-profile-settings-dialog-plans (Back-Button Tour), open Create profile from the switcher
using the keyboard. Tab to Emojis and press Enter, then Tab to Search emojis and type book.
The results expose named grid cells such as Open book. Tab moves to the skin-tone button and
then directly to Cancel, skipping every result. Return to search with Shift+Tab and press
ArrowDown: focus remains in search. This first observation did not inspect virtual selection or
try Enter; the fresh verification below narrows the finding.

The operator completes reading-room with its original sparkles/Green identity, so this is a
unfinished customization path rather than a failure to create any profile. UDS and reload retain
that starter identity. No mouse, programmatic focus or source inspection was used in the walk.

## Evidence

Cycle receipts: profile-create-name-error-sol-emoji-search.json,
profile-create-name-error-sol-emoji-grid.json, profile-create-name-error-sol-emoji-arrow.json,
profile-create-name-error-sol-created.json and profile-create-name-error-sol-ended.json.
Screenshot: profile-emoji-grid-keyboard-unreachable.png. Recording profile-create-name-error-sol
is stopped (138 frames). The original-profile and stale-name feedback repairs pass independently.
Spoken VoiceOver behavior remains unverified; this finding is established by actual keyboard focus.

## Fresh verification narrows the symptom

The picker uses virtual selection while focus stays in its search input. On a fresh dialog,
searching book exposes Notebook with decorative cover as selected, and a polite live region
contains that name. ArrowRight twice still leaves the same result selected. Enter successfully
commits it to the identity preview. Two further ArrowRight presses and ArrowDown retain the
first active result. Sol creates book-notes with that emoji; UDS confirms 📔 and reload retains it.

The claim that no emoji is keyboard-selectable is disproven. The remaining finding is arrow
navigation that does not advance from the first result; its cause remains under investigation.
No production change has been made to the picker. Registry id is retained to preserve history.

Receipts: profile-emoji-keyboard-sol-*.json. Screenshots:
profile-emoji-keyboard-enter-observed.png and profile-book-notes-persisted.png. The owned profile
is 01M40JJA9S8W4P11CYHK227FJB, #c26ad6, 📔. Recording profile-emoji-keyboard-sol is stopped
(55 frames). No source or database reads occurred during this fresh attempt; only the browser
driver's key-dispatch helper was inspected. Spoken screen-reader output remains unverified.

## Root cause and owning regression

E2E-014 now searches book in the real identity dialog, presses ArrowRight to Closed book and
Open book, commits with Enter and checks the saved emoji in the public mutation response. Before
the repair it fails at Closed book, matching the persona walk with an independent keyboard driver.
Receipt: profile-emoji-keyboard-e2e-before.log; failed screenshot/video are preserved under
profile-emoji-keyboard-e2e-before-artifacts/. The existing Settings E2E suite owns this composition
boundary; no duplicate primitive test or new suite is added.

Base UI Dialog.Popup stops propagation of composite keys, including arrows. Frimousse 0.3.0
listens for navigation on document, so its first result and Enter selection work while arrows
never reach their owner. Both profile identity dialogs now use Base UI's documented
[event customization](https://base-ui.com/react/handbook/customization) to let arrow events reach
that owner. Other keys, including Escape, keep the dialog's handling. This is confined to the
two existing SymbolPicker dialog compositions; shared modal behavior and dependencies are unchanged.
The application does not synthesize replacement events or add a second navigation implementation.
## Fresh fixed replay

Sol re-enters through the switcher, types shelf-notes, searches book and reaches Closed book and
Open book with ArrowRight. Enter selects the latter. Escape closes Create and restores focus to
Profile: book-notes; an independent UDS lookup returns profile_not_found for shelf-notes.

Through Settings, both archived profiles and the selection map initially remain collapsed. Sol
opens Edit reading-room, chooses Open book with arrows/Enter, and saves. UDS retains the original
profile id and Green color while reporting 📖 and no icon. Reload shows the new identity; reopening
the editor starts in Emojis with the stored color. Escape returns focus to the original edit button.
The initial Settings navigation wait used button instead of the observed link role; this selector
error is retained separately from the successful keyboard walk.

Receipts: profile-emoji-fixed-sol-*.json. Screenshots: profile-emoji-fixed-create-selection.png,
profile-settings-default-read.png, profile-emoji-fixed-identity-selection.png and
profile-emoji-fixed-settings-persisted.png. Recording profile-emoji-fixed-sol is stopped (149 frames).
No source or database reads occurred during the replay. VoiceOver speech remains unverified.

The unchanged regression now passes with adjacent E2E-013 (2 cases, 19.6 seconds), proving real
daemon persistence through the identity response. Root Turbo build/typecheck and React Doctor
100/100 over 39 changed files also pass. Receipts: profile-emoji-keyboard-e2e-after.log,
profile-emoji-keyboard-build.log and profile-emoji-keyboard-react-doctor.log. Delivery gate and
commit receipts follow separately. Full Settings/switcher charters remain Pending.
