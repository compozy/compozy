# Accessibility Floor

Read this when implementing or verifying any interactive widget, especially dialogs, menus, comboboxes, tabs, sliders, and any custom composite control. WCAG 2.2 AA is the floor. Ceiling is AAA where it can be reached without harming clarity.

## Non-Negotiables (WCAG 2.2 AA)

Every interactive surface must satisfy these without exception:

### Contrast
- Body text: **≥ 4.5:1** against its background
- Large text (≥ 18.66px bold or ≥ 24px regular): **≥ 3:1**
- Non-text UI elements (icons, focus rings, state indicators, form borders that convey state): **≥ 3:1**
- Disabled controls have no minimum contrast in WCAG, but they must still be distinguishable from active controls by a non-color cue
- Verify against the actual rendered RGB, not against the token name. Tokens drift; pixels do not.

### Keyboard Operability
Every interactive element must be:
- **Reachable** via Tab (or Shift+Tab in reverse)
- **Operable** via Enter, Space, or the WAI-ARIA-defined key for its role
- **Dismissible** for popovers, modals, menus — via Esc, click-outside, or both
- **Tab order** matches visual reading order (left-to-right, top-to-bottom in LTR locales). Never use `tabindex` values > 0.

### Focus Visibility
- `:focus-visible` styling is required on every interactive element
- Outline ≥ 2px solid (or equivalent ring), with ≥ 3:1 contrast against the element and against the page background
- Offset the outline from the element (`outline-offset: 2px` or similar) so it is not hidden by hover states
- Never style `:focus-visible` identically to `:hover` — keyboard users lose the indicator
- Never `outline: none` without replacing it. A removed focus ring is a removed lifeline.

### Target Size
- Touch surfaces: **≥ 44×44 CSS pixels**
- Desktop surfaces: **≥ 24×24 CSS pixels** (WCAG 2.2 minimum) — but ≥ 32×32 is a more humane floor
- Tap-target spacing: at least 8px between independent targets to prevent accidental activation

### Motion
- Respect `prefers-reduced-motion: reduce` — replace transforms/translates with opacity-only or no motion
- Auto-playing animation longer than 5 seconds must have a pause/stop control
- Parallax, autoscroll, and infinite spinners must respect the user's motion preference

### Semantic Structure
- One `<h1>` per page (the page's title)
- Headings in nested order — never skip levels (`h2` → `h4` is wrong)
- Landmarks: `<header>`, `<nav>`, `<main>`, `<aside>`, `<footer>` — never re-implement them as `<div role="navigation">` if the semantic tag works
- Lists are lists (`<ul>`, `<ol>`, `<dl>`), not styled divs
- Buttons are buttons (`<button>`), links are links (`<a href>`). Never use a `div` with `onClick`.

### Form Semantics
- Every input has a programmatic label — `<label for>`, `aria-labelledby`, or `aria-label`
- Required fields are programmatically marked (`required` attribute) AND visually marked (asterisk or text)
- Error messages are programmatically associated via `aria-describedby` to the field that errored
- Blocking errors use `role="alert"` or `aria-live="assertive"` to interrupt; non-blocking validation uses `aria-live="polite"`
- Placeholders are not labels — they disappear, they have low contrast, and they vanish on input

## ARIA Component Patterns

For composite widgets, follow the WAI-ARIA Authoring Practices. Critical patterns:

### Dialog (Modal)
- Container: `role="dialog"` (or `role="alertdialog"` for irreversible/blocking)
- Labeled via `aria-labelledby` pointing to the title, optionally `aria-describedby` to a short description
- **Focus trap:** focus moves into the dialog on open, cycles within, returns to the trigger on close
- Esc closes the dialog (unless `alertdialog` requires explicit choice)
- Background is inert: `aria-hidden="true"` on the rest of the app, or use `<dialog>` element with native modal behavior
- Click on the backdrop optionally dismisses (but never as the only way out)

### Combobox / Autocomplete
- Input has `role="combobox"`, `aria-expanded`, `aria-controls` pointing to the listbox, `aria-activedescendant` pointing to the highlighted option
- Listbox has `role="listbox"`, options have `role="option"` and `aria-selected`
- Arrow keys move highlight; Enter selects; Esc closes; Home/End jump to first/last
- Filtering is announced via `aria-live` if results change dramatically

### Menu / Menubar
- `role="menu"` for popout menus, `role="menubar"` for persistent menu bars
- Items: `role="menuitem"`, `role="menuitemcheckbox"`, `role="menuitemradio"`
- Arrow keys move focus between items; Esc closes; Enter activates
- Submenus open on right-arrow (LTR) / left-arrow (RTL), close on the opposite arrow

### Tabs
- Tablist: `role="tablist"`, each tab: `role="tab"` with `aria-selected` and `aria-controls`
- Panels: `role="tabpanel"` with `aria-labelledby` pointing to its tab
- Arrow keys move between tabs (Home/End jump to ends); Tab moves from tablist into the active panel
- **Automatic activation** (focus = select) for simple tab sets; **manual activation** (focus + Enter/Space to select) when switching has cost

### Slider
- `role="slider"` with `aria-valuemin`, `aria-valuemax`, `aria-valuenow`, optionally `aria-valuetext`
- Arrow keys adjust by step; Home/End jump to min/max; Page Up/Down for larger jumps

### Disclosure (Accordion, Expand/Collapse)
- Trigger: `<button>` with `aria-expanded` and `aria-controls`
- Panel: visible/hidden by attribute (not just CSS `display: none` that strips it from the a11y tree if also `visibility: hidden`)

### Toast / Snackbar
- `role="status"` (polite) or `role="alert"` (assertive) — pick based on urgency
- Do not require interaction to dismiss; auto-dismiss for non-actionable toasts; if actionable, give enough time (8s minimum) or do not auto-dismiss

## Reduced Motion Strategy

Detect once, branch everywhere:

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
    scroll-behavior: auto !important;
  }
}
```

This is a floor, not a ceiling. For higher craft:
- Replace transforms with opacity fades (still motion, but vestibular-safe)
- Replace marquees and autoscrolls with static layouts
- Disable parallax entirely
- Keep state transitions (color, opacity) — they help comprehension; remove motion (position, scale) — they cause discomfort

## Color and Meaning

- **Never use color as the sole carrier of meaning.** Red text alone is not enough to signal error — pair with an icon and the word `Error` or with a border, position, or structural cue
- **Form validation:** red border + error icon + text — three signals, not one
- **Status badges:** color + icon + label — three signals, not one
- **Charts:** color + texture/pattern + label — color-blind users (≈8% of men) need the structural cue

## Internationalization Pressure on A11y

- Translations are typically 30% longer in German, French, Portuguese; 40% in Russian. Test with worst-case strings.
- RTL languages reverse layout entirely. Mirror icons that indicate direction (arrows, back/forward), keep brand marks fixed.
- Dates, numbers, currencies follow locale rules — use `Intl.*` APIs, never hand-format.

## Verification Recipes

### Manual audit (per surface)
1. **Tab through** the entire surface. Every interactive element must receive visible focus in reading order.
2. **Screen reader smoke test** — VoiceOver on macOS (Cmd+F5) or NVDA on Windows. The page should be navigable by landmarks, headings, and form fields without seeing the screen.
3. **Color blindness simulation** — Chrome DevTools → Rendering → Emulate vision deficiencies. Cycle through all four modes; confirm all meaning still conveyed.
4. **Zoom test** — Cmd/Ctrl+plus until 200% browser zoom. Layout must remain usable with no horizontal scrolling on body content (WCAG 1.4.10 reflow).
5. **Reduced motion** — toggle in OS or DevTools; confirm vestibular-triggering motion is suppressed.
6. **Contrast** — DevTools picker on every text/icon color pair. Confirm against the actual rendered RGB.

### Automated assistance (not a replacement)
- axe DevTools (browser extension) catches ~30% of issues — landmarks, missing labels, contrast, ARIA misuse
- Lighthouse a11y score is a directional signal, not a verdict
- Storybook a11y addon catches issues per-component during dev
- Linting: `eslint-plugin-jsx-a11y` catches common JSX mistakes before they ship

**Automated tools do not detect:**
- Whether labels are meaningful
- Whether focus order matches reading order
- Whether ARIA roles match actual behavior
- Whether motion is appropriate
- Whether color carries unique meaning

These require human verification.

## Common Failures in AI-Generated Markup

When an LLM produces UI code, audit specifically for:

- `<div onClick={...}>` instead of `<button>` — not keyboard-operable, not in the a11y tree
- `<a>` without `href` used as a button — not focusable
- Custom dropdowns without `role`, `aria-expanded`, `aria-controls` — invisible to AT
- Missing `<label>` on form inputs — placeholders treated as labels
- `<img>` without `alt` — or with decorative `alt="image"` instead of `alt=""`
- Color-only state (red border, green border) — no icon, no text cue
- Removed focus outline without replacement — `outline: none` with no `:focus-visible`
- Tab indices > 0 — out-of-order focus traversal
- `aria-hidden="true"` on a focusable element — gone for screen readers, still tabbable
- Modal without focus trap — Tab escapes to the inert background

## Sources

- W3C — Web Content Accessibility Guidelines (WCAG) 2.2 (Recommendation, October 2023)
- W3C — WAI-ARIA Authoring Practices Guide (APG) — pattern definitions and keyboard contracts
- WebAIM — Contrast Checker, Keyboard Accessibility, ARIA techniques
- U.S. Section 508 — federal accessibility requirements (effectively WCAG 2.0 AA)
- The A11y Project — practical checklist, hiding-content guide, inclusive components
