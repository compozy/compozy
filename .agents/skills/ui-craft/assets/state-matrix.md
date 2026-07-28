# Component State Matrix

Copy this template per component. Fill every row that applies. Cross out rows that genuinely do not (e.g. `hover` on a touch-only surface) with a one-line justification — never silently drop.

## Component: <name>

**Job (one sentence, user-perspective):** <e.g. "Lets the user confirm and trigger an irreversible delete on their project.">

**Variants:** <e.g. `primary`, `secondary`, `destructive`>

**Sizes:** <e.g. `sm` (32px), `md` (40px), `lg` (48px)>

| State              | Visual treatment                                                 | Content                                  | Accessibility                                                        | Animation                                       | Exit / next state                              |
| ------------------ | ---------------------------------------------------------------- | ---------------------------------------- | -------------------------------------------------------------------- | ----------------------------------------------- | ---------------------------------------------- |
| **default**        |                                                                  |                                          |                                                                      | n/a                                             |                                                |
| **hover**          |                                                                  |                                          | Skip on touch surfaces                                               | 120ms color transition                          | → default on mouse-out                         |
| **active / pressed** |                                                                |                                          |                                                                      | Instant                                         | → hover or default                             |
| **focus-visible**  |                                                                  | Same as default                          | Outline ≥ 2px, contrast ≥ 3:1, offset 2px                            | Instant                                         | → default on blur                              |
| **disabled**       |                                                                  |                                          | Contrast ≥ 3:1 against background. Explain `why` if non-obvious      | n/a                                             | → default when re-enabled                      |
| **loading**        |                                                                  |                                          | Disable trigger. Preserve width. `aria-busy="true"`                  | Indeterminate spinner OR determinate progress   | → success / error                              |
| **empty**          |                                                                  | Explain + primary action                 |                                                                      |                                                 | → populated on first item                      |
| **selected / on**  |                                                                  |                                          | Non-color signal (icon, weight, position) + color                    | 120ms                                           | → unselected                                   |
| **error**          |                                                                  | What happened + how to recover           | `aria-live="polite"` or `role="alert"` per urgency                   | Optional shake — respects reduced-motion        | → recovery action                              |
| **success**        |                                                                  | Optional confirmation                    | `role="status"` if announced                                         | 240ms                                           | Auto-dismiss for transient, persist for audit  |
| **<custom>**       |                                                                  |                                          |                                                                      |                                                 |                                                |

## Cross-state requirements

- [ ] Every interactive state has visible focus-visible
- [ ] Every transition between states respects `prefers-reduced-motion: reduce`
- [ ] Every state passes contrast check at its rendered colors
- [ ] Every state announces correctly to a screen reader (test with VO or NVDA)
- [ ] No state introduces layout shift relative to default

## Notes

<Use this section to document any non-obvious decisions — e.g. why `hover` was skipped on a touch-first surface, or why `loading` preserves an exact width for a button.>
