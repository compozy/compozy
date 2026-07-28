# Pre-Ship Checklist

Binary gate. Run on every UI change before merge or PR. Any unchecked box is a blocker, not a "nice to have."

## Dial settings declared

Before reviewing the rest of the surface, record the dial values you designed against. Reviewers compare against these — not against their own taste.

- [ ] `VISUAL_VARIANCE` declared: ____ (1–10, default 6)
- [ ] `MOTION_INTENSITY` declared: ____ (1–10, default 4)
- [ ] `INFORMATION_DENSITY` declared: ____ (1–10, default 5)
- [ ] **Register declared:** Product / Brand
- [ ] **Scene sentence written:** _________________________________________

## Design system fidelity
- [ ] DESIGN.md read and respected
- [ ] All colors reference semantic tokens — no raw hex in component code
- [ ] All spacing snaps to the scale — no magic numbers
- [ ] All radii drawn from the radius scale — no `border-radius: 7px`
- [ ] All shadows drawn from the elevation scale — no ad-hoc box-shadows
- [ ] All durations and easings drawn from the motion scale
- [ ] New tokens (if any) appended to DESIGN.md and the token file in the same change set

## State completeness
- [ ] State matrix filled for every new or changed component
- [ ] Default, hover, active, focus-visible designed
- [ ] Disabled state designed with ≥ 3:1 contrast and a `why` cue if non-obvious
- [ ] Loading state designed — disables trigger, shows progress signal, preserves layout
- [ ] Empty state designed — explains category, offers primary action
- [ ] Error state designed — plain language, specific cause, concrete recovery
- [ ] Success state designed (or transient confirmation per UX)

## Accessibility floor (WCAG 2.2 AA)
- [ ] Body text contrast ≥ 4.5:1 at rendered RGB
- [ ] Large text contrast ≥ 3:1
- [ ] Non-text UI / state indicators contrast ≥ 3:1
- [ ] Every interactive element has visible `:focus-visible` (≥ 2px outline, ≥ 3:1 contrast, offset)
- [ ] All interactive elements reachable by Tab in reading order
- [ ] Keyboard contracts per WAI-ARIA pattern for composite widgets (dialog, menu, combobox, tabs)
- [ ] Touch targets ≥ 44×44 (mobile), desktop targets ≥ 24×24
- [ ] `prefers-reduced-motion: reduce` honored — no vestibular-triggering motion
- [ ] Headings in nested order, one `<h1>` per page
- [ ] Landmarks present (`<header>`, `<nav>`, `<main>`, `<aside>`, `<footer>`)
- [ ] Form inputs have programmatic labels
- [ ] Form errors associated via `aria-describedby`, announced via `aria-live` or `role="alert"`
- [ ] No `<div onClick>` posing as a button; no `<a>` without `href` posing as a link
- [ ] Color never the sole carrier of meaning (always paired with icon, shape, or text)

## Anti-slop scan
- [ ] No VisualSameness — hierarchy clear at a squint
- [ ] No WeakHierarchy — one primary action per view
- [ ] No TextOverflow — tested with worst-case content + i18n
- [ ] No FakeInteractivity — affordance matches behavior
- [ ] No EmojiSpam — emojis absent from product chrome
- [ ] No GradientCrutch — gradients reserved for brand moments
- [ ] No GlassmorphismAbuse — blur on at most one elevated surface
- [ ] No GenericIllustration — text-only or branded empty states
- [ ] No DesignSystemDrift — no raw hex, no magic spacing
- [ ] No StateMatrixHoles — every state designed
- [ ] No CenteredEverything — body text left-aligned
- [ ] No RandomRadii — radius hierarchy consistent
- [ ] No GhostFocus — `:focus-visible` styled
- [ ] No MagicNumbers — values snap to scales

## Microcopy quality
- [ ] CTAs use verb + object (`Delete project`, not `OK`)
- [ ] Errors say what happened, why, and how to recover
- [ ] No banned AI vocabulary (`elevate`, `seamless`, `unleash`, `journey`, `delve`, etc.)
- [ ] No empty greetings (`Welcome!`, `Hi there!`, `Let's get started!`)
- [ ] No filler (`Please`, `Note that`, `In order to`)
- [ ] No blame language pointed at the user
- [ ] No performed delight on routine actions
- [ ] Tone matches the stakes (neutral for routine, plain for destructive, factual for errors)

## Responsive and platform
- [ ] Tested at 320px / 768px / 1280px / 1920px viewport widths
- [ ] No horizontal scroll on body content at any supported breakpoint
- [ ] Dark mode verified for every state (not just default)
- [ ] Mobile touch targets and gestures verified

## Internationalization
- [ ] Tested with strings 30–40% longer than English (German / French / Portuguese sample)
- [ ] Numeric and date formatting via `Intl.*` APIs, not hand-rolled
- [ ] Layout would survive RTL flip (icons mirror, content reflows)

## AI-feature surfaces (if applicable)
- [ ] User can tell what the AI can do (Microsoft G1)
- [ ] User can tell how well it does it (G2)
- [ ] Easy to trigger and dismiss (G7, G8)
- [ ] Easy to correct or refine output (G9)
- [ ] Sources / reasoning surfaced (G11)
- [ ] Uncertainty visible when meaningful
- [ ] Error recovery flow tested
- [ ] User can disable the feature (G17)

## Documentation
- [ ] DESIGN.md updated if tokens / variants / patterns introduced
- [ ] Component docs or Storybook updated if the design system layer changed
- [ ] Rules files (`CLAUDE.md`, `.cursorrules`) still accurate after the change

## Verification evidence

Provide one or more of:
- Screenshots at each breakpoint (default state at minimum)
- Screenshot of each non-default state in the state matrix
- Keyboard navigation recording or step-by-step description
- Screen reader test result (VO / NVDA pass)
- Contrast checker output for any state-pair changed (run `scripts/check-contrast.mjs --json tokens.json` for token batches)
- Token-drift scan output (`scripts/detect-token-drift.mjs <source-dir>` — must exit 0)
- Performance evidence: Lighthouse CI run, Core Web Vitals snapshot, or bundle analyzer delta

## Severity rollup

Tally hits against the Anti-slop scorecard (see `SKILL.md`):

- Critical: ____ — **must be 0** to merge
- Serious:  ____ — **must be 0** to pass review approval
- Moderate: ____ — ≥ 3 on a single surface compounds to Serious (treat as such)

If Critical or Serious > 0, surface is not shippable. No "we'll fix it next sprint." File the issue, do the fix, re-run the checklist.

---

If any box is unchecked at PR time, either fix it or document explicitly in the PR description why this change is shipping with the gap — owner, tracking issue, and unblock criteria. Silent unchecked boxes are slop debt.
