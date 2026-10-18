# Motion reference

Load this when an artifact adds or changes interface motion. The project's motion tokens, component library, and approved references own personality and exact values; the numbers below are starting points when none exist. The reduced-motion and flashing rules are accessibility requirements, not style preferences. Video, motion graphics, and timeline compositions are out of scope.

## Decide whether it animates

Motion earns its place when the user moves through space, time, or state: navigation, container expansion, progress, and gesture follow-through. Research that equalizes static and animated material finds no comprehension gain from animation used to teach, so do not animate to decorate, signal "premium," or fill silence.

| Frequency                                                 | Decision                      |
| --------------------------------------------------------- | ----------------------------- |
| Hundreds of times a day, or any keyboard-initiated action | No animation                  |
| Tens of times a day (hover, list navigation)              | Nearly imperceptible, or none |
| Occasional (modal, drawer, toast)                         | Standard motion               |
| Rare or first-time (onboarding, success)                  | Room for expressive motion    |

Name the purpose before writing code: feedback, spatial consistency, state indication, bridging a jarring change, explanation on marketing or onboarding surfaces, or delight at the rare tier. If no purpose applies, change state instantly and keep a static cue. Data the user is reading or acting on should not move for style.

## Choose the cheapest tool

- A CSS transition handles hover, press, color, and class- or attribute-driven state.
- `@starting-style` handles entry on insertion without script state.
- A CSS animation handles predetermined motion that must stay smooth while the page is busy.
- `element.animate()` provides programmatic control without a library.
- Reach for a motion library only for springs, layout animation, exit animation, or gesture-driven values, and only when the artifact can still open locally without a remote runtime.

Transitions retarget from the current value; keyframes restart. Use transitions for anything a user can trigger repeatedly, such as toasts and toggles.

## Properties and geometry

- Animate `transform` and `opacity`; `clip-path` is also inexpensive. Avoid animating `width`, `height`, `margin`, `padding`, `top`, or `left`, except height for an accordion without a transform equivalent. Name properties instead of `transition: all`.
- Never enter from `scale(0)`. Start from roughly `scale(0.95)` with `opacity: 0`.
- Anchor `transform-origin` at the trigger for popovers, menus, dropdowns, and tooltips. Centered modals are exempt.
- Prefer percentage translation, which follows the element's own size, over fixed pixel distances.
- Set a child's transform directly instead of deriving it from a frequently changing parent custom property.

## Easing, duration, and springs

| Situation                           | Easing      |
| ----------------------------------- | ----------- |
| Entering or exiting                 | ease-out    |
| Moving or morphing on screen        | ease-in-out |
| Hover or color change               | ease        |
| Constant motion (marquee, progress) | linear      |

Do not use ease-in for interface motion; its slow start delays the moment the user is watching. Built-in keywords are weak for deliberate motion. When the project has no curve tokens, start from `cubic-bezier(0.23, 1, 0.32, 1)` for ease-out, `cubic-bezier(0.77, 0, 0.175, 1)` for on-screen movement, and `cubic-bezier(0.32, 0.72, 0, 1)` for drawers. Take other curves from an established easing source rather than inventing one.

| Element                              | Starting duration                     |
| ------------------------------------ | ------------------------------------- |
| Press or hover feedback              | 50–160 ms                             |
| State confirmation                   | About 150 ms                          |
| Tooltip or small popover             | 125–200 ms                            |
| Dropdown or select                   | 150–250 ms                            |
| Modal, sheet, or drawer              | 200–300 ms, up to 500 ms for a drawer |
| Screen transition or container morph | 300–500 ms                            |

Keep frequent interactions at or under about 200 ms and other interface motion under 300 ms unless the change crosses screens or stages content. Mobile travel distances are shorter; durations can be 20–30% shorter than desktop. A 180 ms dropdown feels faster than a 400 ms one.

Use a curve between two known values such as opacity and color. Use a spring for drag with momentum, interruptible or reversible gestures, and physical position or scale changes. Frameworks disagree about what "default" spring means, so choose damping consciously. Keep bounce around 0.1–0.3 and reserve it for drag-to-dismiss or playful moments.

## Interruption, exit, and sequence

- Exit along the path of entry: a toast that enters from the bottom leaves through the bottom.
- Use asymmetric timing where the user decides: slow on the deliberate phase, such as a hold-to-confirm, and fast on the system response.
- Stagger groups by about 30–80 ms instead of entering everything at once, and keep the total sequence short.
- Gate hover-only motion with `@media (hover: hover) and (pointer: fine)` so taps do not trigger false hovers.
- Pause ambient or looping motion when its surface is hidden or offscreen, and prefer one-shot celebration over loops.

## Reduced motion and flashing

Ship reduced-motion behavior with the animation. Under `prefers-reduced-motion: reduce`, remove translation, scale, rotation, and parallax, and keep a static state cue or a gentle opacity or color crossfade when a change still needs conveying. Reduced motion means fewer and gentler animations, not removing comprehension. The View Transitions API does not apply this preference automatically. Nonessential automatic movement lasting over five seconds alongside other content needs pause, stop, or hide. Never exceed three flashes in any one-second period.

## Check the motion

Read the CSS and script for the rules above. When feel matters and rendered inspection is available, slow the animation down or step through frames. Report motion feel that could not be observed as unverified instead of estimating it from code.
