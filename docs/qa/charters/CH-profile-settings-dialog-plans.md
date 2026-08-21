# CH-profile-settings-dialog-plans: Operate every profile dialog by keyboard and escape out of each one

```yaml
charter:
  id: CH-profile-settings-dialog-plans
  mission: "As Sol, reach the switcher, the symbol picker, and every Settings lifecycle dialog with the keyboard and a screen reader alone, confirming each dialog renders exactly what its plan endpoint returned and announces state without relying on colour — then back out of each one at every step and prove nothing partial was left behind."
  mode: charter-with-tour
  persona:
    name: Sol
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-profiles
  scenarios: [ET-profile-switcher-restore, ET-profile-web-settings-lifecycle-dialogs]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Reach and operate the switcher with the keyboard only: it must be arrow-navigable, announce the active profile and the All-profiles state, and carry the boundary sentence verbatim. Confirm the quiet single-profile state announces nothing about profiles at all, and that after a second profile exists the identity is announced by name and symbol, never by colour."
      - "Open the symbol picker: traverse the icons and emojis tabs, the search field, the grid, and the colour row by keyboard; confirm the grid is announceable, an invalid hex is reported inline while the previous colour stands, and the auto-assigned starter is described rather than merely shown."
      - "Walk each lifecycle dialog and compare it against its plan payload: rename tiers (machine informational, repo offers pre-checked, placements going dormant), archive's paused-automation list and its blocked-by-running-session warning, unarchive's reactivation list, and delete's enumeration or its routing to archive. Make one plan stale from a terminal and require the dialog to re-ask rather than execute."
      - "Back out of everything: Escape from each dialog, browser Back from a dialog and from a success screen, Back after a switch, and a reload mid-dialog. Focus must return somewhere sensible, no dialog may be left stuck open, no partial profile or partial rename may survive, and every validation error must be reported inline against its field rather than as a toast."
    must_avoid:
      - "Settling the aggregate presentation (CH-profile-aggregate-owner-truth owns it) or palette handoff internals (CH-profile-palette-lens-isolation); default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest and a production-parity web build."
```

## Selection rationale

Two contracts meet in these dialogs. The first is accessibility: `_uiux.md` requires the switcher to
be fully arrow-navigable, the picker keyboard-searchable, dialogs to trap focus, and the destination
chip to be text — and colour is user data here, so a profile's identity colour can never be the
thing that communicates state. Sol is the only persona who falsifies that rather than assuming it.
The second is Safety Invariant 15: prepare is strictly read-only and a plan-backed mutation never
commits against a stale plan. A dialog that renders anything its plan did not return, or that
executes a revision it no longer holds, breaks the lifecycle guarantee at the surface layer rather
than the store layer. The Back-Button Tour is the matrix's first choice for Settings and is exactly
the pressure that exposes a half-open dialog or a partially applied rename.

## Evidence and entry points

- **Web** — screenshots of the quiet and plural switcher, the picker's four states, the Settings page default read, and each dialog state including its refusals; a keyboard-only and screen-reader pass note covering the switcher, the picker grid, and dialog focus traps.
- **HTTP** — the rename, archive, and delete plan payloads beside the dialog that rendered them, and the mutation quoting its revision plus the stale-plan refusal.
- **CLI** — the terminal transcript used to make the plan stale and to change the profile out from under an open client.
- **UDS and agent** — none required; the parity of these mutations is settled by CH-profile-lifecycle-plan-recovery.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
