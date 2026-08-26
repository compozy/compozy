# CH-terminal-profile-fence: Seed two profiles with terminals and try to make one see the other

```yaml
charter:
  id: CH-terminal-profile-fence
  mission: "As Ada, open terminals under two profiles in one project, then attack every terminal read the daemon offers with hostile and ambiguous scope values, trying to make one foreign terminal, journal row, pending request, badge count, or stream frame appear where it must not — and to get a terminal created from a session that has no project at all."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-switch-profile-terminal-scope
  scenarios: [ET-terminal-profile-selectors, ET-terminal-profile-segmentation]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Seed both profiles: running terminals, journal history, a pending input request, a recording, a spill artifact, and a typing grant. A surface with no fixture is an unprobed surface — say so in the debrief."
      - "Feed every read hostile scope values: an unknown profile, an archived one, an empty one, one differing only in case, both selectors at once, the aggregate form on a change verb, and the aggregate against a single-owner read. Each must be a typed refusal or an empty result, never unfiltered rows."
      - "From inside a session, pass a profile that contradicts the session's own and confirm the session vetoes rather than the flag overriding; then from a session with no project, try to open and to execute and confirm both are refused with the no-project reason and nothing appears afterwards."
      - "Switch profile with the Terminal app open and confirm the list, tab strip, dock badge, catalog stream, and journal all re-scope; address the hidden terminal by id and confirm it reads as absent with no hint of an owner; then switch back and confirm the process is still running with its scrollback."
      - "Try to use a typing grant issued under one profile from the other; fill one profile's per-project budget and confirm the other can still open terminals; then archive a profile and confirm its terminals close while its journal rows and recordings stay readable, and finally delete the project and confirm both disappear."
      - "Compare the same query across the command line, both transports, the catalog stream, and a native read from inside a session; all five must agree on scope and on owner labels."
    must_avoid:
      - "Do not settle the general profile lens for non-terminal domains — CH-profile-foreign-leak-probe owns that; use the isolated lab from the bootstrap manifest and never write configuration concurrently against one isolated home."
```

## Focus areas

- **Safety Invariant 18 (profile scoping fails closed)** — its own focus area. Every read resolves scope
  through the shared resolver; an unresolvable or invalid scope compiles to an empty result, never an
  unfiltered one; changes are never aggregate; and every event payload declares its profile and can be
  matched by a profile-scoped subscriber. A payload that cannot be matched is a silent isolation break,
  not a visible failure — probe it directly.
- **Safety Invariant 19 (a caller with no workspace is refused, not guessed)** — its own focus area. A
  session with no project asking for a terminal gets the no-project refusal; the daemon never falls back
  to a remembered project. Verify on creation and on execution, through the agent tools and both
  transports.
- **Safety Invariant 9 (every registry operation is workspace- and profile-fenced)** — a mismatch on any
  component behaves as absence, ownership is immutable, and the per-project cap counts per profile while
  the installation-wide cap stays global.
- **ADR-017 (terminals are profile-owned and profile-filtered)** and **ADR-018 (profile integrity
  enforced in code)** — the record lives in a different store from the profile, so the owner link is
  enforced rather than guaranteed by the database.
- **Archive and delete asymmetry** — archiving closes terminals but preserves history; deleting the
  project removes both.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
