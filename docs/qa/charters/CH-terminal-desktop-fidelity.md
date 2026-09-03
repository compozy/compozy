# CH-terminal-desktop-fidelity: Type into the packaged desktop terminal the way a real terminal user does

```yaml
charter:
  id: CH-terminal-desktop-fidelity
  mission: "As Marina working in the packaged desktop app, paste, compose, copy, zoom, and resize inside the terminal until something arrives wrong — a doubled keystroke, a lost composition, a stale cell after a full-screen program, or a socket the shell should never have allowed."
  mode: charter-with-tour
  persona:
    name: Marina
    device: desktop
    network: wifi-fast
    locale: ja-JP
  journey: J-use-terminal-desktop
  scenarios: [APP-terminal-desktop-fidelity]
  tour: Paste Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Paste multi-line text, text with hidden formatting from a word processor, tab-separated text from a spreadsheet, and a very long single line; confirm each arrives as the shell would expect and nothing is submitted that you did not see."
      - "Compose Japanese input through the input method and confirm exactly one intended sequence reaches the program — no doubled character, no committed fragment, no lost composition on window blur."
      - "Use the native edit menu and its keyboard shortcuts for copy and paste, and confirm they do not collide with the application's own shortcuts."
      - "Zoom in and out, then resize the window, and compare the controlling view against a watching view of the same terminal; both must reflow to the same screen."
      - "Run a full-screen program, resize it, then exit it; confirm the previous screen, cursor, and scrollback return with no stale cells left behind."
      - "Confirm the terminal socket is accepted from the app's own origin and refused from any other, within a bounded time rather than hanging."
    must_avoid:
      - "Do not settle browser-only terminal behaviour — CH-terminal-operator-shell owns that; do not change the shell's network policy to make a probe pass, a refusal is the expected result."
```

## Focus areas

- **Same-origin socket policy** — the packaged shell admits only its own terminal socket; a cross-origin
  probe is refused promptly instead of hanging. Widening this policy is a security decision, not a test
  fix.
- **Input fidelity** — clipboard, native accelerators, and input-method composition each deliver exactly
  one intended sequence; the known desktop input risks are accelerator collision, clipboard permission,
  composition, and zoom refit.
- **Rendering fidelity** — controller and watcher reflow identically, and leaving a full-screen program
  restores the previous screen and cursor without stale cells.
- **ADR-015 (pinned terminal renderer, daemon-authoritative sizing)** — the daemon decides the size and
  the renderer follows it; zoom must not desynchronise the two.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
