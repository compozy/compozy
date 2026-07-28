# Tool-call grouping and thread accessibility regression

- Legacy ID: AB-007
- Source: J-14 / RT-048, RT-049, RT-055..RT-057, RT-060 / `_tests.md` §8.2–§8.9 visual; `_qa.md` §6 J-D flag
- Why automate: the transcript derive layer (grouping, `+N previous tool calls`, settled-turn collapse, structural sharing) is stable enough to pin; row accessibility is unit-owned with no axe pass over the assembled thread.
- Suggested layer: unit (pure-logic timeline) + an axe/accessibility sweep of the redesigned thread.
- Spec sketch: assert grouping folds consecutive tool calls, the `+N` toggle reveals prior calls, settled turns collapse to `Worked for Xs`, and structural sharing preserves row identity across refetch; axe-check status-not-color-only, labelled toggles, and reduced motion. True end state: the visual-language invariants and accessibility floor hold as the derive layer evolves.
- Status: proposed
