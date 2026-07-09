# UI Audit — <surface name>

**Auditor:** <agent or person>
**Date:** <YYYY-MM-DD>
**Scope:** <route(s), component(s), or page(s) under review>
**Tokens of authority:** <DESIGN.md path | tokens.css path | CLAUDE.md path>

## Summary

<2–3 sentences. Overall verdict: shippable / fix-before-ship / rework. Lead with the single highest-impact finding.>

## Findings (by severity)

### Blockers
*These prevent ship. Fix is non-negotiable.*

| # | Pattern              | Where (file:line / selector)     | Evidence                       | Fix                                  |
| - | -------------------- | -------------------------------- | ------------------------------ | ------------------------------------ |
| 1 |                      |                                  |                                |                                      |

### Risks
*High-impact issues that should be fixed before ship if budget allows.*

| # | Pattern              | Where                            | Evidence                       | Fix                                  |
| - | -------------------- | -------------------------------- | ------------------------------ | ------------------------------------ |
| 1 |                      |                                  |                                |                                      |

### Nits
*Low-impact issues worth noting for the cleanup pass.*

| # | Pattern              | Where                            | Fix                                  |
| - | -------------------- | -------------------------------- | ------------------------------------ |
| 1 |                      |                                  |                                      |

## Anti-slop Scorecard Hits

For each of the 14 patterns in `references/ai-slop-patterns.md`, mark presence + severity.

| #  | Pattern              | Hit? | Severity (B/R/N) | Note                                           |
| -- | -------------------- | ---- | ---------------- | ---------------------------------------------- |
| 1  | VisualSameness       |      |                  |                                                |
| 2  | WeakHierarchy        |      |                  |                                                |
| 3  | TextOverflow         |      |                  |                                                |
| 4  | FakeInteractivity    |      |                  |                                                |
| 5  | EmojiSpam            |      |                  |                                                |
| 6  | GradientCrutch       |      |                  |                                                |
| 7  | GlassmorphismAbuse   |      |                  |                                                |
| 8  | GenericIllustration  |      |                  |                                                |
| 9  | DesignSystemDrift    |      |                  |                                                |
| 10 | StateMatrixHoles     |      |                  |                                                |
| 11 | CenteredEverything   |      |                  |                                                |
| 12 | RandomRadii          |      |                  |                                                |
| 13 | GhostFocus           |      |                  |                                                |
| 14 | MagicNumbers         |      |                  |                                                |

## State Coverage

For each component in scope, fill the state-matrix audit:

| Component         | default | hover | active | focus-visible | disabled | loading | empty | error | success |
| ----------------- | ------- | ----- | ------ | ------------- | -------- | ------- | ----- | ----- | ------- |
| <Button>          |    ✓    |   ✓   |   ✓    |       ✓       |    ✓     |    ✗    |  n/a  |  n/a  |   n/a   |
| <DataTable>       |         |       |        |               |          |         |       |       |         |

Legend: ✓ designed, ✗ missing, n/a not applicable

## Token Drift

| Property        | Raw value in code  | Token that should be used   | Where                            |
| --------------- | ------------------ | --------------------------- | -------------------------------- |
| color           | `#3b82f6`          | `--color-action-primary`    | `Button.tsx:23`                  |
| spacing         | `11px`             | `--space-3` (12px) or scale | `Card.tsx:45`                    |
| radius          | `7px`              | `--radius-md` (8px)         | `Input.tsx:12`                   |

## Microcopy Issues

| Where                            | Current                          | Issue                              | Suggested                            |
| -------------------------------- | -------------------------------- | ---------------------------------- | ------------------------------------ |
| `SubmitButton.tsx:18`            | `Submit`                         | Generic CTA                        | `Save changes`                       |
| `EmptyState.tsx:5`               | `Welcome! No data yet 🎉`        | Greeting + emoji + no action       | `No projects yet. Create your first.`|

## Accessibility Failures

| Failure                                       | Severity | Where                            | Fix                                                |
| --------------------------------------------- | -------- | -------------------------------- | -------------------------------------------------- |
| Focus indicator missing                       | Blocker  | `Tabs.tsx`                       | Add `:focus-visible` outline                       |
| Color contrast 3.8:1 on body text             | Blocker  | `Card.tsx body`                  | Use `--color-text-primary` (passes)                |
| `<div onClick>` instead of `<button>`         | Blocker  | `ActionRow.tsx:34`               | Convert to `<button>`, keyboard support free       |

## Verification Evidence

- [ ] Tab-through complete; focus order matches reading order
- [ ] Screen reader smoke test (VO or NVDA) passes per surface
- [ ] Reduced-motion verified
- [ ] Contrast verified at rendered RGB for body, secondary, disabled
- [ ] Tested at 320px / 768px / 1280px viewport widths
- [ ] Tested with worst-case content lengths
- [ ] Dark mode verified for every state

Attach screenshots in `<surface>-evidence/` and link below:
- <screenshot of default at desktop>
- <screenshot of loading state>
- <screenshot of empty state>
- <screenshot of error state>
- <screenshot of focus-visible at keyboard>
- <screenshot of dark mode>
- <screenshot of mobile 360px>

## Recommended Diff (optional)

For high-priority blockers, sketch the change as a unified diff or numbered edit list:

```diff
- <div onClick={handleDelete} className="text-red-500">
+ <button type="button" onClick={handleDelete} className="...">
```

## Sign-off

- [ ] All blockers resolved or accepted with documented rationale
- [ ] Risks have an owner and a tracking issue
- [ ] DESIGN.md updated if new tokens / variants introduced
- [ ] State matrix complete for every component touched
- [ ] Pre-ship checklist (`assets/pre-ship-checklist.md`) passed
