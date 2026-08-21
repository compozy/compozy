# CH-palette-domain-views-grammar: Walk every view kind keyboard-only with the domain grammar honest

```yaml
charter:
  id: CH-palette-domain-views-grammar
  mission: "As Sol, keyboard-only with a screen reader, browse the palette's domain views across all four kinds — chips with truthful counts, detail previews, typed forms, 2D grids — proving every state is announceable, reachable, and honest: no color-only badge, no false empty, no silent truncation, no vault value."
  mode: charter-with-tour
  persona:
    name: Sol
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-command-os-from-palette
  scenarios: [ET-palette-domain-views]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open the Tasks view keyboard-only: chips single-select with truthful counts, the zero-match empty state names the filter and clears with one keystroke, and every state badge pairs glyph and label the screen reader can announce."
      - "Traverse a detail pane (focus stays in the list, pane scrolls independently), a form (declared field order, first-invalid focus on blocked submit, masked password), and the marketplace grid (←→↑↓ across sections, ⏎/panel parity, placeholder on broken media) — all under the same stack/Esc ladder."
      - "Open a cold-cache domain (loading, never false empty), a domain past its mount cap (exact 'showing N of M' or full scroll), and the Vault view (names and metadata only — no value anywhere, including match highlights)."
      - "Verify the ARIA combobox contract at root and in views: input keeps DOM focus, arrows move the active row not focus, and live refreshes never steal selection."
    must_avoid:
      - "The Sessions exemplar's landing semantics (CH-palette-sessions-landing-canary owns them); extension program views (isolation charter owns them)."
```

## Selection rationale

Targeted tier: ADR-004's full List/Detail/Form/Grid vocabulary and US-010..013 shipped 13 domain
views and three new view kinds in task 06 with no prior scenario owner. Sol is the mandated persona
— the palette's ARIA combobox pattern, WCAG 2.1.4 shortcut rules, and never-color-alone are spec
accessibility contracts, and keyboard-only is the palette's core promise.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
