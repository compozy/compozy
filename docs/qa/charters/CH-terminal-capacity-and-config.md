# CH-terminal-capacity-and-config: Change every terminal setting, then run into every limit it sets

```yaml
charter:
  id: CH-terminal-capacity-and-config
  mission: "As Dora, read and change all ten terminal settings at every scope they allow, then deliberately hit each limit they define and check that the refusal names the blocking resource, the current usage, and a recovery an operator can actually perform."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-terminal-capacity
  scenarios: [MS-terminal-config-lifecycle, ET-terminal-limits-capabilities]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Read the global, project, and profile projections and record all ten defaults with where each value came from; then set each key to a valid value at every scope it allows and open a new terminal after each change to see when it takes effect."
      - "Submit one invalid value for every validation path and confirm the refusal names the offending key and leaves the effective value unchanged; then try to raise the installation-wide cap from a profile overlay and confirm it cannot be done."
      - "Navigate away from the terminal settings mid-edit and come back, reload the settings surface, and use the browser back button after saving; confirm no stale value is shown as saved and no saved value is silently reverted."
      - "Fill the per-project terminal budget and confirm the refusal lists the existing terminal identifiers and routes to settings; then fill the viewer budget from another client and confirm only the excess viewer is refused while existing viewers keep their streams."
      - "Recover capacity — close a terminal, disconnect a viewer, raise an allowed cap — and confirm the next attempt succeeds without disturbing unrelated terminal work."
      - "Open a terminal in an execute-only workspace and confirm command output is still available while interactive controls and interactive capability claims are absent rather than present-but-disabled."
    must_avoid:
      - "Do not write configuration concurrently against one isolated home — run the changes one at a time per scope; do not settle the platform matrix itself, CH-terminal-platform-ladder owns it."
```

## Focus areas

- **ADR-016 (`[terminal]` configuration; policy stays in permissions)** — ten documented keys with sane
  defaults, validated on load, projected into settings, and no autonomy policy among them.
- **Configuration lifecycle** — every key layers across global, project, and profile scope, states when
  it applies, and refuses an invalid value by naming the key rather than failing vaguely.
- **Admission control** — the per-project and viewer budgets fail closed with the blocking resource, the
  current usage, the existing terminal identifiers, and a recovery route; a profile overlay can never
  raise the installation-wide cap.
- **Capability honesty (ADR-004)** — an execute-only workspace shows no interactive control and claims no
  interactive capability; absent, not disabled.
- **Truthful-UI directive** — the settings surface never offers a control the runtime cannot honour.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
