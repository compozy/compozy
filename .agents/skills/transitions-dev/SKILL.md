---
name: transitions-dev
description: Production-ready CSS transitions for web apps. Use when implementing notification badges, dropdowns, modals, panel reveals, page transitions, card resizes, number pop-ins, text swaps, icon swaps, success checks, avatar group hovers, error state shakes, search/input clear, skeleton loaders, shimmer text, sliding tabs, tooltips, staggered text reveals, card hover tilt, plus-to-menu morph, or accordions. Triggers on "add a transition", "animate the dropdown", "make the modal open smoothly", "swap icon", "page slide", "stagger animation", "open / close transition", "make it animate", "fade between", "success animation", "form error", "shake on invalid", "hover lift", "avatar stack hover", "clear the search", "skeleton loader", "loading shimmer", "shimmer text", "sliding tabs", "segmented control", "tooltip", "reveal text", "tilt card", "3D hover tilt", "cursor glare", "plus to menu", "FAB morph", "accordion", "collapsible", "expand / collapse", "disclosure", "replace this transition", "which transition fits here". Also the commands transitions reveal and transitions apply. For auditing or polishing existing motion against the motion-token scale ("motion tokens", "review my animations", "tune the durations", "align to the motion tokens"), use the companion transitions-polish skill.
---

# Transitions.dev

Twenty-one portable CSS transitions, each namespaced under `t-*` selectors with semantic CSS custom properties. Drop-in: paste the snippet, wire the documented HTML hooks, done. No framework dependencies, no demo-specific markup, and every snippet ships a `prefers-reduced-motion` guard.

## Quick reference

| Transition | When to use | Reference |
| --- | --- | --- |
| **Card resize** | Tween a container's width or height when its layout state changes. | [01-card-resize.md](./01-card-resize.md) |
| **Number pop-in** | Re-enter each digit with a blurred slide when a number updates. | [02-number-pop-in.md](./02-number-pop-in.md) |
| **Notification badge** | Slide a small badge onto a trigger and pop the dot. | [03-notification-badge.md](./03-notification-badge.md) |
| **Text states swap** | Swap text in place with a blurred up-and-down transition. | [04-text-states-swap.md](./04-text-states-swap.md) |
| **Menu dropdown** | Open an origin-aware dropdown that grows from its trigger. | [05-menu-dropdown.md](./05-menu-dropdown.md) |
| **Modal open / close** | Scale-up modal dialog with a softer scale-down on close. | [06-modal.md](./06-modal.md) |
| **Panel reveal** | Slide a panel into a region with a cross-blur. | [07-panel-reveal.md](./07-panel-reveal.md) |
| **Page side-by-side** | Slide between two side-by-side pages (list ↔ detail, step 1 ↔ step 2). | [08-page-side-by-side.md](./08-page-side-by-side.md) |
| **Icon swap** | Cross-fade two icons in the same slot with blur and scale. | [09-icon-swap.md](./09-icon-swap.md) |
| **Success check** | Compose fade + rotate + Y-bob + path stroke-draw to celebrate a completed action. | [10-success-check.md](./10-success-check.md) |
| **Avatar group hover** | Distance-falloff lift on a row of items with a bouncy spring on return. | [11-avatar-group-hover.md](./11-avatar-group-hover.md) |
| **Error state shake** | Per-segment cubic-bezier shake with auto-reverting border + message. | [12-error-state-shake.md](./12-error-state-shake.md) |
| **Input clear with dissolve** | Fly-out + per-word streak when a text field is cleared. | [13-input-clear-dissolve.md](./13-input-clear-dissolve.md) |
| **Skeleton loader and reveal** | Pulse a placeholder, then cross-fade + cross-blur to the loaded content. | [14-skeleton-reveal.md](./14-skeleton-reveal.md) |
| **Shimmer text** | Sweep a highlight band across muted text on a loop (pure CSS). | [15-shimmer-text.md](./15-shimmer-text.md) |
| **Tabs sliding** | Slide the active pill between tabs in a segmented control. | [16-tabs-sliding.md](./16-tabs-sliding.md) |
| **Tooltip open/close** | Delayed fade+scale in, instant out (pure CSS). | [17-tooltip.md](./17-tooltip.md) |
| **Texts reveal** | Staggered blurred rise for stacked text lines, quiet fade out. | [18-texts-reveal.md](./18-texts-reveal.md) |
| **Card hover tilt** | Tilt a card in 3D toward the pointer with a cursor-tracked glare. | [19-card-tilt.md](./19-card-tilt.md) |
| **Plus to menu morph** | Morph a circular trigger into the menu / panel it opens. | [20-plus-menu-morph.md](./20-plus-menu-morph.md) |
| **Accordion expand** | Grow / shrink a panel via grid-rows with a chevron flip. | [21-accordion.md](./21-accordion.md) |

## Decision rules

When the user asks for a transition, match against the visible UI element first, then the verb:

- **Trigger + small dot floating on top** → notification badge.
- **Trigger + surface that grows from it** → dropdown (anchored, origin-aware) or modal (centered, no anchor).
- **Surface that slides into a region of the page** → panel reveal.
- **Two screens, list ↔ detail or step 1 ↔ step 2** → page side-by-side.
- **Element changes width or height** → card resize.
- **Element's text content changes in place** → text states swap.
- **Two icons in the same slot** → icon swap.
- **A number updates** → number pop-in.
- **Confirmation / success / "done" moment** (checkmark, payment processed, file uploaded) → success check.
- **Hovering an item in a horizontal stack** (avatars, chips, segmented buttons, tag pills) → avatar group hover.
- **Form validation error / "this is wrong" feedback** (invalid field, wrong PIN, duplicate name) → error state shake.
- **Clearing a text field** (search box × button, filter reset) → input clear with dissolve.
- **Placeholder that loads then swaps to real content** (list row, card, profile header) → skeleton loader and reveal.
- **In-progress / "thinking" text that should feel alive** (loading label, streaming status) → shimmer text.
- **Small horizontal set of mutually-exclusive options with a moving highlight** (view switcher, segmented control, filter tabs) → tabs sliding.
- **Hover/focus hint that appears over a trigger** (icon tooltip, info bubble) → tooltip open / close.
- **Stacked headline + supporting line entering with rhythm** (hero copy, empty state, onboarding step) → texts reveal.
- **Card / tile that should react in 3D to the pointer on hover** (product card, cover art, membership card, with or without a light glare) → card hover tilt.
- **Circular trigger that becomes the surface it opens** (+ FAB grows into a menu / panel, compose button expands) → plus → menu morph. If the surface is a *separate* popover that merely grows from the trigger, use menu dropdown instead.
- **Header with a collapsible body that grows / shrinks in height** (settings group, FAQ, filter section, "show more", disclosure) → accordion expand.
- **No clear match** → fall back to `transitions reveal` and let the user pick. Don't guess.

If two transitions could fit, prefer the lower-overhead one (card resize over panel reveal, dropdown over modal, success check over a full modal celebration) unless the design clearly calls for the heavier surface. The success check is animation-only — if you also need to swap from a spinner to the check, pair it with **icon swap**.

## Commands

The skill exposes two namespaced verbs the agent should recognise in addition to direct transition requests. Every command starts with `transitions` so the invocation never collides with verbs from other skills installed in the same project. (For auditing or applying motion-token polish on transitions that already exist, the companion **transitions-polish** skill provides `transitions review` and `transitions polish`.)

### transitions reveal — list every transition

**Trigger phrases:** `transitions reveal`, "reveal the transitions", "list all transitions", "what transitions are in this skill", "show the transitions catalog".

**Behaviour:** print the twenty-one transitions as a numbered plain-text list — name, one-line summary, and the matching reference filename. Reuse the rows in `## Quick reference` above; do not invent new copy. No project access.

### transitions apply — install the best-fit transition

**Trigger phrases:** `transitions apply`, "apply a transition here", "add the right transition", "install transitions-dev here", "fix the animation on this element".

**Behaviour:**

1. Read context: the currently-open file, the element nearest the cursor, surrounding CSS / JSX. If the user named a transition explicitly (e.g. `transitions apply menu-dropdown`), use it.
2. Run the decision rules from `## Decision rules` on that context and pick **one** transition. If two could fit, prefer the lower-overhead one (same tie-breaker the existing rules use).
3. Surface a one-line proposal: "I'd apply **menu-dropdown** here because the element opens from a trigger and is anchored. Confirm to install?".
4. On confirmation, follow the existing five-step procedure in `## Output format` verbatim (root block, snippet, hooks, reduced-motion guard, JS orchestration if needed).
5. If the agent can't pick a single transition with confidence, fall back to `transitions reveal` and ask the user to choose.

## Universal install

Copy [`_root.css`](./_root.css) into your project **once** and import it (or paste its `:root` block into your global stylesheet). It holds the semantic tunable variables for **all twenty-one** transitions. Every snippet reads from these names — `--resize-*`, `--badge-*`, `--dropdown-*`, `--clear-*`, `--shimmer-*`, `--tabs-*`, `--tt-*`, `--stagger-*`, `--tilt-*`, `--morph-*`, `--acc-*`, and the rest. The snippets ship literal values, so they work on their own; to tune them against a shared motion-token scale, install the companion **transitions-polish** skill.

Each reference file also restates just the variables that snippet needs, so you can install a single transition without pulling the whole block. Don't duplicate the block — if `_root.css` is already imported, skip re-pasting any per-snippet `:root`.

The `--pX-*` source tokens used by the live demo at [transitions.dev](https://transitions.dev) are intentionally **not** exported. Tunable values are renamed to semantic names so the user owns the design vocabulary. A few transitions (input clear, shimmer text, tabs, tooltip) carry **color** tokens that differ by theme — each reference file documents the `html[data-theme="dark"]` overrides.

## Output format

When inserting a transition into the user's project:

1. **Install the variables from `_root.css`** into the user's global stylesheet, but only if they aren't already there — or just the per-snippet `:root` block from the reference file if installing a single transition. If the universal block is already imported, do **not** duplicate it.
2. **Paste the chosen transition's CSS verbatim** from the relevant reference file. Do not rewrite selectors, do not collapse the transition into shorthand, do not strip `will-change`. The snippets are tuned and tested.
3. **Wire the documented HTML hooks** — class names (`.t-dropdown`, `.t-modal`, `.t-success-check`, `.t-avatar`, `.t-clear`, `.t-skel`, `.t-shimmer`, `.t-tabs`, `.t-tt`, `.t-stagger`, `.t-tilt`, `.t-morph`, `.t-acc`, …) and state attributes (`data-open`, `data-state`, `data-page`, `data-origin`, `aria-selected`, `aria-expanded`, `.is-open`, `.is-closing`, `.is-error`, `.is-shaking`, `.has-value`, `.is-clearing`, `.is-pulsing`, `.is-revealed`, `.is-shown`, `.is-hiding`, `.is-hover`, `.is-tilting`).
4. **Preserve the `@media (prefers-reduced-motion: reduce)` block.** Every snippet ships one. Removing it makes the component fail accessibility audits.
5. **For transitions that need JS** (dropdown, modal, text swap, number pop-in, page slide, success check, avatar group hover, error state shake, input clear, skeleton reveal, tabs sliding, texts reveal, card hover tilt, plus → menu morph, accordion expand), copy the small orchestration snippet from the reference file and adapt the selectors to the user's DOM. Keep the timing reads (`getComputedStyle(...)getPropertyValue("--…")`) so durations stay in sync with the `:root` values. Shimmer text and tooltip are **pure CSS** — no JS needed.

Keep the diff small: only edit the files needed to introduce the transition. Don't rename the user's existing variables, don't reformat unrelated CSS, don't pull in a motion library.

## Common mistakes to avoid

- **Stripping the close-state class cleanup** on dropdown/modal — without the `setTimeout` that removes `.is-closing`, the next open jumps from the closing scale instead of the resting pre-open scale.
- **Forgetting the reflow** in the text swap, number pop-in, success check replay, and error state shake — `void el.offsetWidth` (or `offsetHeight`) between class/attribute removal and re-addition is what guarantees the animation replays.
- **Animating a single container** instead of the inner pieces — for the badge, animate the dot, not the trigger; for page slide, animate the page sections, not the container.
- **Replacing `transition: …` with `transition: all`** — every snippet enumerates exact properties on purpose so unrelated style changes don't ride in for free.
- **Hardcoding the success check's `stroke-dasharray`** — the snippet ships `20` as a placeholder. Replace it with `path.getTotalLength()` rounded up by 1 for *your* path, otherwise the stroke pre-reveals or over-draws.
- **Setting `transition-timing-function` in CSS** for the avatar group hover — it has to be set inline in JS *before* the `--shift` / `--scale-active` writes so the bouncy ease-out only applies on `mouseleave`.
- **Mixing `.is-error` and `.is-shaking` into one class** for the error state shake — keeping them orthogonal is what allows the shake to replay (remove → reflow → re-add) without flickering the whole error treatment.
- **Leaving the input clear glow on `mix-blend-mode: multiply` in dark mode** — flip to `screen`, bump `--glow-opacity` to ~0.85, and paint white gradients in JS.
- **Forgetting to write the tabs pill's first position without a transition** — on first paint and resize, set `transform` + `width` with `transition: none` (then reflow + restore) or the pill animates in from `translateX(0)` / `width: 0`.
- **Tracking the pointer on the tilting element itself** for card hover tilt — bind `pointermove` to the flat outer `.t-tilt` wrapper, not `.t-tilt-card`, or the rotating edges slip under the cursor and the hover flickers.
- **Padding on the accordion grid track** — put padding on `.t-acc-panel-inner`, never on `.t-acc-panel`; padding on the `0fr` track leaves a residual height strip so the panel never fully closes.
- **Morphing the accordion chevron's `d` path** — CSS `d:` path interpolation is Chromium-only, so it never animates on mobile Safari / Firefox. Flip the chevron vertically (`transform: scaleY(-1)`) instead — it passes through a flat line at the midpoint just like the path morph and works everywhere. Keep the path symmetric about its viewBox centre and add `vector-effect: non-scaling-stroke` so the stroke stays constant through the flip. This is what the snippet ships.

## Reference files

- [01-card-resize.md](./01-card-resize.md) — Card resize
- [02-number-pop-in.md](./02-number-pop-in.md) — Number pop-in
- [03-notification-badge.md](./03-notification-badge.md) — Notification badge
- [04-text-states-swap.md](./04-text-states-swap.md) — Text states swap
- [05-menu-dropdown.md](./05-menu-dropdown.md) — Menu dropdown
- [06-modal.md](./06-modal.md) — Modal open / close
- [07-panel-reveal.md](./07-panel-reveal.md) — Panel reveal
- [08-page-side-by-side.md](./08-page-side-by-side.md) — Page side-by-side
- [09-icon-swap.md](./09-icon-swap.md) — Icon swap
- [10-success-check.md](./10-success-check.md) — Success check
- [11-avatar-group-hover.md](./11-avatar-group-hover.md) — Avatar group hover
- [12-error-state-shake.md](./12-error-state-shake.md) — Error state shake
- [13-input-clear-dissolve.md](./13-input-clear-dissolve.md) — Input clear with dissolve
- [14-skeleton-reveal.md](./14-skeleton-reveal.md) — Skeleton loader and reveal
- [15-shimmer-text.md](./15-shimmer-text.md) — Shimmer text
- [16-tabs-sliding.md](./16-tabs-sliding.md) — Tabs sliding
- [17-tooltip.md](./17-tooltip.md) — Tooltip open/close
- [18-texts-reveal.md](./18-texts-reveal.md) — Texts reveal
- [19-card-tilt.md](./19-card-tilt.md) — Card hover tilt
- [20-plus-menu-morph.md](./20-plus-menu-morph.md) — Plus to menu morph
- [21-accordion.md](./21-accordion.md) — Accordion expand
- [_root.css](./_root.css) — the universal install block on its own, ready to import directly.
