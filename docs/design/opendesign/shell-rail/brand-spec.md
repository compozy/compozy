# Shell v2: brand spec (source: aicss.dev "Approval Card")

Values were extracted verbatim from the component's stylesheet (`--ab-*` tokens, `:root/[data-theme=light]`
and `[data-theme=dark]`). The dark theme uses them exactly. The light theme uses the same system's light set.

One sentence: neutral, chroma-free greys on two surface levels (card and sunken inset), pill actions with
a single inverted primary, Inter at weight 425, and mint for "in progress" and brand wells, and Compozy orange for highlights.

| Role | Dark | Light | Source token |
|---|---|---|---|
| chrome (rail, topbar) | #0a0a0a | #fafafa | ab-bg / ab-stage-bg |
| surface (panes, cards) | #1a1a1a | #fff | ab-surface |
| sunken (tab strip, inset panels) | #101010 | #f7f8fa | ab-bg-subtle |
| surface-2 (hover, secondary pill) | #242424 | #f4f5f7 | ab-surface-2 |
| border / strong | #303030 / #424242 | #e6e8ec / #d4d7dd | ab-border(-strong) |
| fg / muted / subtle | #f5f5f5 / #a3a3a3 / #737373 | #1a1a1a / #a1a1a1 / #a1a1a1 | ab-text* |
| primary / on-primary | #f5f5f5 / #0a0a0a | #0b0d12 / #fff | ab-accent / ab-on-accent |
| highlight / needs-you (Compozy) | #e8572a | #d14e25 | Compozy accent (overrides the reference) |
| success · warning · danger | #34d399 · #f5b14c · #f87171 | #15a06a · #d98404 · #dc2626 | ab-success… |
| shadow-card / elevated | verbatim | verbatim | ab-shadow-card / -elevated |

Derived with the reference's own recipes: `fg-2 = mix(muted 60%, fg)` and `fg-3 = mix(fg 70%, muted)`.

## Type
Inter for UI, weight 425 / 500 / 600, tracking −0.01em. Geist Mono for code, paths and terminal.
Scale: 12 · 13 · 13.5 · 14.5 (body) · 15 (titles, medium) · 17 (headings, medium).

## Rules observed
1. **Two surface levels.** Content sits on a card surface, and secondary lists sit in a sunken inset panel
   with no border.
2. **All actions are pills.** Secondary actions use `surface-2`. The single primary is inverted (white on
   dark, black on light) and may carry a ↵ key hint.
3. **Icon wells.** A 26px well with radius 7 and a mint tint marks a surface's identity.
4. **State glyphs.** A mint spinner ring means in progress, a dashed ring means queued, a filled mint check
   means done, and an orange dot means needs you. The amber is only for real warnings.
5. **Muted is quiet.** Secondary copy uses `muted`, and hovers only step the surface (`surface-2`) or add
   `shadow-card`. They never dim the text.
6. **Readable greys (production override).** `muted`, `subtle` and `faint` carry text, so production lifts
   them to WCAG AA (≥4.5:1) instead of the reference's values. Light: `muted`/`subtle`/`faint` #a1a1a1 → #6e6e6e
   (2.6:1 → 4.8:1 on sunken), and `fg-2`/`fg-3` re-derive from it with the same recipes (#4c4c4c / #333333).
   Dark: `subtle` #737373 → #8e8e8e and `faint` → #8a8a8a (≥4.5:1 on canvas, canvas-tint, rail and sunken).
   `muted` in dark (#a3a3a3) already passes. This is a deliberate accessibility override. The prototype
   keeps the reference values.
