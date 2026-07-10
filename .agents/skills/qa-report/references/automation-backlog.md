# Automation Backlog

Automation intent lives in **one** doc — `<qa-docs-path>/automation-backlog.md` — never as metadata attached to scenarios or charters. "Should this become an automated E2E spec?" is an engineering-planning question; asking it on every session note drags the planner back into coverage-matrix thinking.

## Contents

- What belongs here
- Entry format
- When to add an entry
- Lifecycle
- Anti-patterns

## What belongs here

- Journeys stable and valuable enough to deserve an automated E2E spec.
- Regression-prone scenarios (repeat offenders in the bug registry) worth pinning.
- Fix-verification replays that had no meaningful automated test at fix time (per the `qa-execution` skill's fix-loop governor) and should eventually get one.

## Entry format

```markdown
## AB-<NNN>: <journey or scenario title>
- Source: <J-NN / state.csv ids / BUG-NNNN that motivated this>
- Why automate: <regression-prone | high-value stable journey | fix lacked a test>
- Suggested layer: <E2E browser | API/integration | unit>
- Spec sketch: <2-4 lines: entry, key assertions incl. true end state>
- Status: proposed | accepted | implemented (<test path>) | rejected (<reason>)
```

Ids `AB-NNN` are monotonic like every id in the tree.

## When to add an entry

- A scenario fails the same way twice across cycles (check the bug's `Re-found`/`Regressed` sections).
- A journey reaches `pass` for three consecutive cycles and is P0 — stable enough to pin, valuable enough to matter.
- A fix shipped with a documented replay instead of a regression test — the entry records the debt.

## Lifecycle

1. Planner or executor appends entries (dedup by `Source` first).
2. Engineering triages: `accepted` or `rejected` with reasoning.
3. When implemented, record the test path and flip to `implemented` — the scenario's future cycles can then downgrade to smoke cadence.

## Anti-patterns

- **Automation fields on scenarios/charters** — `automation_target`, `automation_status` per artifact is the old model; it pulls every planning conversation into tooling.
- **Backlog as a dumping ground** — an entry without a motivating source (journey, bug, fix) is noise.
- **Automating instead of walking** — an implemented spec covers regressions of a known path; it never replaces persona sessions for new or changed surfaces.
