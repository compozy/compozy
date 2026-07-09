# Motion Patterns

Read this when adding any animation, transition, or motion to a UI surface — and when reviewing motion that "feels off" but the cause is not obvious. Motion is a load-bearing channel: it communicates causality, hierarchy, and state. Done well, comprehension speeds up. Done badly, the UI feels cheap and the user feels patronized.

`visual-craft.md` covers when to animate. This file covers how — with code.

## The 100 / 300 / 500 Rule

Most motion in a product fits into four duration bands. Memorize these; do not eyeball.

| Use                                            | Duration   | Examples                                                       |
| ---------------------------------------------- | ---------- | -------------------------------------------------------------- |
| Micro-feedback (hover, focus, press)           | 100–150ms  | Button hover, focus ring fade, input border tint               |
| State change (toggle, accordion, tab swap)     | 200–300ms  | Checkbox tick, switch slide, accordion expand                  |
| Layout move (modal entry, side sheet, drawer)  | 300–500ms  | Dialog reveal, drawer slide, route transition                  |
| Entrance / brand moment (hero reveal, splash)  | 500–800ms  | Landing hero, onboarding step, celebration                     |

**Exit ≈ 75% of entry.** A 320ms entrance pairs with a ~240ms exit. The mind expects departure to feel decisive.

Anything > 800ms in functional UI is a smell. The user is now waiting, not perceiving.

## Easing

Easing is more important than duration. Wrong curve, right duration → still feels cheap.

### Choose by intent

| Curve                         | When                                                              |
| ----------------------------- | ----------------------------------------------------------------- |
| `ease-out` (decel)            | Entrances. Object settling into place. Most product motion.       |
| `ease-in` (accel)             | Exits. Object leaving the viewport. Departures.                   |
| `ease-in-out` (s-curve)       | State swaps that stay (tab content, theme toggle).                |
| Linear                        | Indeterminate loaders, parallax, continuous motion only.          |

### Quality curves (replace browser defaults)

The CSS keywords `ease-out` etc. are mild. Reach for exponential curves for premium feel.

```css
:root {
  /* The good stuff — use these */
  --ease-out-quart:  cubic-bezier(0.25, 1, 0.5, 1);
  --ease-out-quint:  cubic-bezier(0.22, 1, 0.36, 1);
  --ease-out-expo:   cubic-bezier(0.16, 1, 0.3, 1);

  --ease-in-quart:   cubic-bezier(0.5, 0, 0.75, 0);
  --ease-in-expo:    cubic-bezier(0.7, 0, 0.84, 0);

  --ease-in-out-quart: cubic-bezier(0.76, 0, 0.24, 1);
  --ease-in-out-expo:  cubic-bezier(0.87, 0, 0.13, 1);

  /* Spring-y, brand moments only */
  --ease-spring-soft: cubic-bezier(0.34, 1.56, 0.64, 1);
}
```

### Banned in product chrome

- **Bounce / elastic / overshoot** anywhere a user is trying to complete a task. It reads as toy-like and dated. Reserve for one-off celebration moments only.
- **Linear on entrances.** It feels mechanical. The eye expects deceleration.
- **`ease`** (the default keyword) — it is symmetric in/out, which is rarely what you want. Pick a side.

## Code Patterns

Three implementation surfaces. Pick by what the project already uses; don't introduce a new motion lib for one transition.

### CSS / Tailwind

The default. Cheap, performant, accessible. Most product motion lives here.

```css
/* Button hover with a quality curve */
.btn {
  transition:
    background-color 120ms var(--ease-out-quart),
    transform 120ms var(--ease-out-quart);
}
.btn:hover { background-color: var(--color-action-hover); }
.btn:active { transform: translateY(1px); }
```

Tailwind 4:
```html
<button class="
  bg-action text-action-foreground
  transition-[background-color,transform] duration-150
  ease-[cubic-bezier(0.25,1,0.5,1)]
  hover:bg-action-hover active:translate-y-px
">Run workflow</button>
```

For Radix / shadcn primitives with `data-state` attributes:
```css
[data-state="open"] .dialog-content {
  animation: dialog-in 240ms var(--ease-out-quint);
}
[data-state="closed"] .dialog-content {
  animation: dialog-out 180ms var(--ease-in-expo);
}

@keyframes dialog-in {
  from { opacity: 0; transform: translateY(8px) scale(0.98); }
  to   { opacity: 1; transform: translateY(0)   scale(1); }
}
@keyframes dialog-out {
  from { opacity: 1; transform: translateY(0)   scale(1); }
  to   { opacity: 0; transform: translateY(8px) scale(0.98); }
}
```

### Framer Motion (now `motion`)

Reach for it when you need: shared layout (`layoutId`), exit animations on unmount (`AnimatePresence`), gesture orchestration (drag/pan), or independent variants per child.

```tsx
import { motion, AnimatePresence } from "motion/react";

<AnimatePresence>
  {isOpen && (
    <motion.div
      key="panel"
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: 8 }}
      transition={{
        duration: 0.24,
        ease: [0.22, 1, 0.36, 1], // quint
      }}
    >
      <Panel />
    </motion.div>
  )}
</AnimatePresence>
```

Shared-layout (the magic that justifies the bundle weight):
```tsx
<motion.div layoutId={`row-${id}`} className="card">…</motion.div>
// When the same layoutId appears in a new tree, framer interpolates between the two positions.
```

**Cost:** ~30kb gz. Do not ship it on a landing page that has one fade.

### CSS @starting-style + transitions (modern)

For `display: none` → `display: block` transitions on dialogs/popovers, the new web standard:
```css
.popover {
  opacity: 1;
  transition: opacity 200ms var(--ease-out-quart), display 200ms allow-discrete;
}
.popover:not([open]) {
  opacity: 0;
  display: none;
}
@starting-style {
  .popover[open] { opacity: 0; }
}
```
Lets the platform handle what Framer used to be required for. Check baseline browser support before relying on it.

## Reduced Motion — Mandatory

Honor `prefers-reduced-motion: reduce`. This is accessibility, not a preference toggle.

```css
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
    scroll-behavior: auto !important;
  }
}
```

This preserves state changes (the button still shows hover; the dialog still opens) while removing motion. It does **not** mean "no transition" — it means "instantaneous transition." A 1ms duration still fires the `transitionend` event, so flows that depend on it keep working.

For Framer Motion:
```tsx
import { MotionConfig, useReducedMotion } from "motion/react";

const reduce = useReducedMotion();
<motion.div animate={{ y: reduce ? 0 : -20 }} />

// Or globally:
<MotionConfig reducedMotion="user">{children}</MotionConfig>
```

## Premium Materials

Transform and opacity alone create competent motion. Premium feel comes from layered materials.

| Material               | When                                                   | Cost                              |
| ---------------------- | ------------------------------------------------------ | --------------------------------- |
| `filter: blur()`       | Hero reveals, modal backdrops, focus-mode chrome dim  | Paint-heavy, one element max      |
| `backdrop-filter`      | Glass surfaces over content                            | Expensive, banned as default      |
| `mask-image`           | Reveal-from-bottom text, gradient fades, scroll edges | Cheap, supports both axes         |
| `clip-path`            | Geometric reveals, brand moments                       | Cheap, animate `inset()` values   |
| `mix-blend-mode`       | Hover effects, hero compositions                       | Paint-heavy on large surfaces     |

Example — text reveal with mask:
```css
.reveal {
  mask-image: linear-gradient(to bottom, black 60%, transparent);
  mask-size: 100% 200%;
  mask-position: 0 100%;
  transition: mask-position 600ms var(--ease-out-expo);
}
.reveal.is-visible {
  mask-position: 0 0;
}
```

## Stagger Patterns

List reveals look generative when items animate together. Stagger by index.

```tsx
<motion.ul initial="hidden" animate="visible" variants={{
  visible: { transition: { staggerChildren: 0.05, delayChildren: 0.1 } },
}}>
  {items.map(item => (
    <motion.li
      key={item.id}
      variants={{
        hidden: { opacity: 0, y: 8 },
        visible: { opacity: 1, y: 0, transition: { duration: 0.24 } },
      }}
    >
      {item.label}
    </motion.li>
  ))}
</motion.ul>
```

Pure CSS:
```css
.list > li {
  opacity: 0;
  transform: translateY(8px);
  animation: list-in 240ms var(--ease-out-quart) forwards;
}
.list > li:nth-child(1) { animation-delay: 0.05s; }
.list > li:nth-child(2) { animation-delay: 0.10s; }
.list > li:nth-child(3) { animation-delay: 0.15s; }
/* … */
```

**Cap stagger at 6 items.** Past that the user is waiting, not enjoying. For long lists, stagger the first 5–6 and snap-in the rest.

## Perceived Performance Tricks

Motion can make slow things feel fast.

- **Optimistic + skeleton crossfade.** Update UI immediately, fetch in background, crossfade real data over the optimistic placeholder when it lands.
- **Preemptive animation start.** Start the dialog animation the moment the user clicks, not when the data finishes loading. The user already committed.
- **Ease-in for waits.** A determinate progress bar with `ease-in` feels shorter than linear — the first 70% flies, the last 30% is the wait.
- **Skeleton-to-content crossfade.** Don't replace skeleton with content suddenly. 120ms opacity crossfade hides the swap.
- **Disable transitions on entry.** When a list mounts, mount items in their final state; transitions trigger only on subsequent state changes. Otherwise every full-page reload animates.

## Anti-Patterns (with severity)

| Pattern                                                           | Severity   |
| ----------------------------------------------------------------- | ---------- |
| Ignoring `prefers-reduced-motion`                                 | Critical   |
| Animating layout properties (`width`, `top`, `margin`)            | Serious    |
| Bounce / elastic in product chrome                                | Serious    |
| Motion with no semantic meaning (decoration only)                 | Serious    |
| Page-load reveal on a product surface                             | Serious    |
| Animating > 1 element simultaneously on the same axis             | Moderate   |
| Linear easing on a state entrance                                 | Moderate   |
| Stagger past 6 children                                           | Moderate   |
| Using `ease` keyword by default                                   | Moderate   |
| Duration > 800ms on a functional interaction                      | Moderate   |

Severity ties back to `SKILL.md`'s Anti-slop scorecard — Critical and Serious block merge.

## Library Choice — Decision Matrix

| Need                                                | Reach for                       |
| --------------------------------------------------- | ------------------------------- |
| Hover, focus, press, simple reveals                 | CSS transitions / Tailwind      |
| State enter/exit on a Radix primitive               | CSS keyframes on `data-state`   |
| Mount/unmount animations on React components        | Framer Motion `AnimatePresence` |
| Shared-element transitions across routes            | Framer Motion `layoutId`        |
| Drag, gesture, pan, scrub                           | Framer Motion or `react-use-gesture` |
| Scroll-linked reveals                               | CSS `animation-timeline: scroll()` (modern) or `framer-motion` `useScroll` |
| Cross-DOM transitions (popovers, dialogs)           | CSS `@starting-style` (modern) |
| Brand-moment hero animation                         | Framer Motion + Lottie if asset-driven |
| 3D / canvas motion                                  | Three.js, Pixi, GSAP — out of scope here |

## Strong Opinion

> Motion without meaning is decoration, and decoration in UX is debt.

Every animation should answer "what state changed and why does the user need to perceive it?" If you cannot answer, delete the animation.

## Sources

- Material Design 3 — *Motion* (physics, easing, durations)
- Apple Human Interface Guidelines — *Motion*
- Cassie Evans — *CSS animations & GSAP talks* on premium feel
- Sarah Drasner — *SVG Animations* (still the canonical text)
- Refactoring UI — chapter on motion as hierarchy
- web.dev/articles/animations-overview — what is and isn't compositor-cheap
- `motion.dev` — Framer Motion / `motion` library docs
