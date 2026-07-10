# Component Patterns

Read this when designing, generating, or reviewing one of: buttons, inputs/forms, dialogs/modals, navigation, tables, cards, dropdown/combobox, tabs, popovers/tooltips, toasts. Each pattern below names the states, the keyboard contract, the a11y requirements, and the most common slop.

Pair this reference with `assets/state-matrix.md` (fill it per component) and `accessibility-floor.md` (a11y floor + ARIA pattern specs).

## Buttons

The most consequential component in any product. Get buttons wrong and the rest of the UI follows.

### Variants (hierarchy)
- **Primary** — one per view. The action the user is most likely to want. Filled with brand color.
- **Secondary** — outlined or subtle filled. Alternative actions of similar weight.
- **Tertiary / Ghost** — text-only. Tertiary actions or low-density toolbars.
- **Destructive** — primary in `danger` color, OR outlined with danger text. Reserved for irreversible operations.
- **Icon-only** — when paired with a tooltip; never the only path to discover an action.

### Sizes
Two or three sizes max:
- **sm** — 28–32px height. Toolbars, table actions.
- **md** — 36–40px height. Default for forms and content.
- **lg** — 44–48px height. Marketing, mobile primaries.

### States (state matrix)
default / hover / active / focus-visible / disabled / loading.

- **Loading state:** swap label for spinner OR show spinner adjacent to label; disable the trigger; preserve width to prevent layout shift
- **Disabled state:** ≥ 3:1 contrast still required for affordance; never disable without an explanation tooltip if the reason is non-obvious

### Keyboard contract
- Activated by Enter AND Space (both — never just one)
- `<button>` element (not `<div onClick>`) — keyboard support is free and correct

### Microcopy
- Verb + object: `Delete project`, `Save changes`, `Send invite`
- Never `OK`, `Submit`, `Click here`, `Continue` (without context)
- Maximum 2–3 words
- Sentence case (`Save changes`) is more humane than Title Case (`Save Changes`) in product UI

### Common slop
- `<a href="#" onClick>` instead of `<button>` — not keyboard-operable, not announced correctly
- Primary action visually identical to secondary — `WeakHierarchy`
- Loading state that does not disable the trigger — double-submit risk
- Icon-only button without `aria-label` or visible tooltip — `FakeInteractivity`

## Inputs and Forms

### Anatomy
```
┌───────────────────────────────────┐
│ Label                  Optional   │  ← label + required/optional cue
├───────────────────────────────────┤
│ Helper text describing the field  │  ← helper, when label is ambiguous
├───────────────────────────────────┤
│ ┌───────────────────────────────┐ │
│ │ user-typed value              │ │  ← the actual input
│ └───────────────────────────────┘ │
├───────────────────────────────────┤
│ ✗ Error message in plain language │  ← only when error
└───────────────────────────────────┘
```

### Rules
- **Label visible always.** Floating labels look slick and fail in practice (low contrast at rest, ambiguous when filled). Default to label-above-field.
- **Required is the norm, mark optional.** When most fields are required, marking required with `*` adds noise. Mark optional fields instead.
- **Helper text under the field, not as placeholder.** Placeholders disappear when the user starts typing — the user loses the hint.
- **Inline validation on blur, not on every keystroke.** Validating mid-typing is hostile. Exception: password strength, character counters.
- **Submit-time validation summary** for long forms — list errors at the top with anchor links to each field.
- **Error messages are specific** — `Email must include @` not `Invalid input`.
- **Group related fields** with shared visual containment (subtle border, background) and a group label.
- **Use the right control:** date picker for dates, select for ≤ 7 options, autocomplete for > 7 options, segmented control for 2–4 mutually exclusive states, switch for instant-effect boolean, checkbox for save-on-submit boolean.

### Keyboard contract
- Tab moves between fields
- Enter submits the form (when focus is on a text input OR the primary submit button)
- Esc cancels (when in a modal form, closes the modal; when in a free-form, clears focus)

### Common slop
- Required indicator only on the label, not programmatically (`required` attribute missing)
- Placeholder used as label — `Recognition over recall` violated
- Error message in a toast instead of inline — user cannot see what field broke
- Long forms with no progress / no step indicators (`Zeigarnik effect` ignored)
- Native HTML validation tooltips left as-is — generic, ugly, inconsistent across browsers; replace with custom inline messages

## Dialogs / Modals

### When to use
- Confirming an irreversible action (delete, leave-without-saving)
- Focused micro-task that interrupts the user (sign in to continue, accept terms)
- Quick form with ≤ 5 fields that does not need a full page

### When NOT to use
- Anything that needs persistence (use a page or side sheet)
- Forms with > 5 fields (use a page or step flow)
- Disclosing information that is not blocking (use a popover or page section)
- Stacking modals (modal-on-modal is almost always a redesign signal)

### Anatomy
- **Title** — names the decision or task (`Delete this project?`, not `Confirm`)
- **Body** — the consequence + relevant context (`All workflows, runs, and logs will be permanently removed.`)
- **Primary action** — verb + object (`Delete project`), right-aligned
- **Secondary action** — `Cancel` to its left
- **Close (X)** in the top-right corner — always available except for blocking `alertdialog`

### Focus management
- Focus moves into the dialog on open (to the first focusable element, or the primary action for confirms)
- Tab cycles within the dialog (focus trap)
- Esc closes (unless `alertdialog` requires explicit choice)
- Focus returns to the trigger element on close

### Destructive confirmation
- Primary button in danger color
- Secondary button is the safe default (`Cancel` first in tab order, OR default focus on Cancel)
- For high-consequence: require typing the resource name (`Type "<project name>" to confirm`)

### Common slop
- Modal without focus trap — Tab escapes to the background
- `alertdialog` semantics on a non-blocking dialog (or vice versa)
- Title is `Confirm` and body is `Are you sure?` — generic and unhelpful
- Backdrop click dismisses a confirmation dialog (accidental data loss vector)
- Modal with > 5 fields — should be a page
- Two primary buttons (Cancel + Delete both filled) — `WeakHierarchy`

## Navigation

### Patterns by product type
- **Top bar** — best for ≤ 7 top-level destinations, content-heavy products
- **Side rail / sidebar** — best for tool-like products with > 5 top-level destinations
- **Bottom tab bar** — mobile only, ≤ 5 destinations
- **Command palette** — accelerator for power users; never the only path to a feature

### Side rail anatomy
- Collapsible (collapsed = icons, expanded = icons + labels)
- Top: brand mark or product switcher
- Middle: primary destinations grouped by domain
- Bottom: secondary (help, settings, profile)
- Current destination always visible (background fill + accent stripe + text weight)

### Keyboard contract
- Tab moves between nav items
- Arrow keys can move within a nav group (optional, but follow if the rest of the UI uses it)
- Each nav item is a real `<a href>` — never `<div onClick>`

### Common slop
- Hidden navigation behind a "hamburger" on desktop — adds a click and hides the structure
- 12+ items at the top level — chunk into groups
- Current location invisible — user does not know where they are
- Nav items that look like buttons (Hick's Law applied wrong)
- Icon-only nav without labels — guessing game

## Tables

### When to use
- Comparing rows of structured data
- Sorting, filtering, batch-acting on records
- Displaying tabular data with relationships

### When NOT to use
- A single item per row that does not need column-wise comparison (use a list)
- Mobile-first surfaces (tables collapse badly — design a card list with the same data)

### Anatomy
- Header row: sticky on scroll, with sort affordance, contrast against body
- Body rows: zebra striping in dense tables; subtle hover state always
- Cell density: comfortable (12–16px row padding) or compact (6–8px), user-toggled
- Right-align numbers and dates; left-align labels
- Truncate with tooltip on hover; never wrap into 4-line cells

### Selection
- Checkbox in the first column
- "Select all visible" vs "Select all matching" distinction (Gmail pattern)
- Bulk action bar appears at the top when ≥ 1 row selected; never a layout shift — overlay on top of the header

### States
- **Empty:** explain what goes in this table and how to add the first row
- **Loading:** skeleton rows matching column structure, not a spinner over the whole table
- **Filtered to zero:** different from empty — say "No matches" + offer "Clear filters"
- **Error:** explain what failed and how to retry, with the previous state preserved when possible

### Common slop
- No skeleton on load — layout shift when data arrives
- Filtered-to-zero state shown as "empty state" — confuses new users
- Header not sticky — user loses orientation
- Numbers left-aligned — column comparison broken
- Tooltips required to read truncated content with no other way to expand

## Cards

### When to use
- Grid of similar entities (projects, runs, items)
- Surface that links to a deeper view per item

### Anatomy
- Padding consistent (typically `space-6` / 24px)
- Title at top, metadata below, primary action in a clear position
- Hover state (subtle elevation, border emphasis) — affordance for clickability
- Clickable area extends to the full card, not just the title; visible focus ring on Tab

### Common slop
- Every card identical — `VisualSameness`
- Card grid that does not respond to content density (long titles overflow or push siblings)
- "Three random emojis at the top" as decoration — `EmojiSpam` / `GenericIllustration`
- Card with two equally-weighted primary actions

## Dropdown / Combobox

### Dropdown (Select)
- Use for: 2–7 mutually-exclusive options. More than 7 → Combobox.
- WAI-ARIA `combobox` or native `<select>` (still the most accessible default on most platforms)
- Selected option visible in the trigger; popover lists the rest
- Esc closes; arrow keys move; Enter selects; typeahead matches

### Combobox (Autocomplete)
- Use for: > 7 options OR users may need to filter/search OR free-text + suggestion
- Listbox with `role="listbox"`, items with `role="option"`
- Async loading: skeleton items + `aria-busy="true"` on the listbox
- Empty filter results: "No matches for "X"" + offer to clear

### Common slop
- Custom dropdown without `role`, `aria-expanded`, `aria-controls` — screen-reader invisible
- No keyboard support — arrows ignored, Enter does nothing
- Trigger that looks identical to a non-clickable label
- Popover that does not respect viewport (clips off-screen)

## Tabs

### When to use
- 2–7 panels of related content within the same context
- User benefits from in-context switching without full navigation

### When NOT to use
- > 7 panels (use navigation)
- Content that needs to be compared side-by-side (use a multi-pane layout)
- Sequential content (use a stepper)

### Anatomy
- Active tab has clear non-color differentiation (underline, fill, or weight)
- Tab labels short, predictable
- One tab is active at all times (no "all unselected" state)

### Keyboard
- Arrow keys move focus between tabs (Home/End jump to ends)
- Tab key exits the tablist into the active panel
- Automatic activation (focus = select) by default; manual activation for expensive panel loads

### Common slop
- Active tab indicated by color only — fails for color-blind users
- Tabs that change height between panels (jarring layout shift)
- Tabs that should be pages (each tab is its own URL and history entry should be a page or query param)

## Popovers and Tooltips

### Tooltip
- **Purpose:** label or one-sentence hint, on hover/focus
- **Triggered by:** hover (after 500ms delay) AND keyboard focus
- **Dismissed by:** mouse-out, blur, Esc
- **Never use for:** critical information, interactive content (links, buttons inside)
- Plain text only; no formatting; max ~80 characters

### Popover
- **Purpose:** secondary information or controls, on click
- **Triggered by:** click
- **Dismissed by:** outside click, Esc, action inside
- **Can contain:** interactive content, forms, links
- Should not be nested (popover-in-popover ≈ redesign)
- Anchored to trigger with visible connection (caret or alignment); auto-flip when near viewport edge

### Common slop
- Tooltip on hover-only — keyboard users excluded
- Tooltip containing critical info — users miss it
- Popover used as a replacement for navigation or dialog
- Tooltip that just repeats the visible label

## Toasts / Snackbars

### When to use
- Transient confirmation (`Saved`, `Copied to clipboard`)
- Non-blocking error or warning (`Could not connect — retrying`)
- Status update from a background process

### When NOT to use
- Anything that requires user response — use a dialog or inline message
- Critical errors that block continuing — use a banner or inline error
- Information the user needs to refer back to (toasts disappear)

### Anatomy
- One line of text, optional action (`Undo`, `View`)
- Auto-dismiss for non-actionable (4–6 seconds); persistent for actionable
- Bottom or top, consistent position throughout product
- Stack vertically when multiple; cap at 3 visible

### A11y
- `role="status"` (polite, for non-urgent) or `role="alert"` (assertive, for urgent)
- Announce only the message text, not the dismiss button
- Pausable on hover/focus; do not dismiss while the user is interacting

### Common slop
- Toast for critical error → user misses it → support ticket
- Toast that disappears in 2 seconds with an `Undo` action no one can click in time
- Toast position changes per surface
- Toast and dialog open simultaneously (toast hidden under modal)

## Skeletons and Loading States

- **Skeleton screens** preferred over spinners for content that has predictable layout
- Skeleton blocks match the final content's rough shape (number of rows, column widths, image presence)
- Spinner only when:
  - Content layout is unknown (search results before count is known)
  - Action is global and short (submitting a form)
- **Optimistic UI** when the action is likely to succeed and reversible — show the result immediately, reconcile on response
- **Progressive rendering** for above-the-fold content — render skeleton + first content stream + remaining content
- Loading state must time out — after ~10s show "Still loading" + retry option; never silent forever

## Empty States

The empty state is a teaching moment, not a placeholder.

### Anatomy
- **Headline** — names the data category and its absence (`No projects yet`)
- **Body** — explains what lives here and why creating one matters
- **Primary action** — the one button that creates the first instance (`Create your first project`)
- **Optional secondary** — link to docs, sample data, or guide

### Common slop
- `No data.` with no explanation, no action
- Generic 3D illustration of "people pointing at laptops" — `GenericIllustration`
- Multiple secondary actions competing — `WeakHierarchy`
- Empty state confused with filtered-to-zero (treat differently)

## Error States

### Inline (per-field)
- Plain language, specific, recovery-oriented
- Anchored to the failing field, never the bottom of the form
- Persists until the user corrects it (do not auto-clear on focus)

### Page-level
- Full-page error only when navigation context is lost (404, 500, auth required)
- Page-level error gives: what happened, why (in plain language), what to do, contact for support
- Never show stack traces in production UI

### Async / background
- Inline banner near the affected content
- Retry mechanism with backoff
- Do not lose user state (preserve form data)

## Sources

- IBM Carbon Design System — Button, Modal, Form, Table patterns
- Atlassian Design System — Modal/dialog examples and accessibility
- Radix UI — Headless accessible primitives (Dialog, DropdownMenu, Combobox, Tabs)
- shadcn/ui — Component composition on Radix + Tailwind
- UK Government Design System — Form patterns, error messages, components for service design
- Polaris (Shopify) — E-commerce-shaped patterns, content guidelines
- WAI-ARIA Authoring Practices Guide — pattern definitions and keyboard contracts
