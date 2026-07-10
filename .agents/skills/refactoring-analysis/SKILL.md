---
name: refactoring-analysis
description: >
  Audits a codebase for refactoring opportunities using Martin Fowler's code smell and
  technique catalog — long functions, duplication, high coupling, complex conditionals,
  primitive obsession — and writes a prioritized report to docs/_refacs/. Use when auditing
  code quality, planning a refactoring sprint, or reviewing architectural health; not for
  style/formatting, performance, or security audits.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Refactoring Analysis

Audit a codebase against Martin Fowler's *Refactoring* (2nd ed.) and produce a prioritized
report. Every finding names a **smell**, cites the real `file:line` you read it at, and maps
to a Fowler **technique** — grounded in code you actually read, never generic advice. When a
step names a reference, read it in full before producing that step's findings.

## Steps

**1. Scope.** Fix the target — directory, module, feature area, or whole project; ask if the
user did not say. Note the language and paradigm (OOP / functional / mixed) to calibrate which
smells apply. If the target exceeds ~50 files, ask to narrow or confirm a sampled scan.
*Done when:* target, language, and paradigm are recorded.

**2. Detect smells.** Scan for every category in `references/code-smells-catalog.md`, using its
heuristics: Bloaters, Change Preventers, Dispensables, Couplers, Conditional Complexity, DRY
Violations. For each hit record: `file:line` range, smell name + category, severity
(`critical` / `high` / `medium` / `low`), and its cost to maintainability, readability, or
change. Flag an ambiguous case as *potential* with the context that might justify it.
*Done when:* every category above has been scanned, not only the ones that fired.

**3. Map techniques.** Give each smell its Fowler technique(s) from the table in
`references/refactoring-techniques.md`, and sketch a concrete before/after for every P0 and P1
finding. *Done when:* each finding carries a named technique.

**4. Assess coupling.** Flag modules with high afferent coupling (many dependents — risky to
change), high efferent coupling (many dependencies — fragile), circular dependencies, and low
cohesion (mixed responsibilities → Extract Class / Split Phase). *Done when:* the dependency
structure is characterized.

**5. SOLID pass (domain projects only).** Apply only when the project is domain-rich — DDD,
hexagonal, or clean architecture; otherwise skip and note why in the report.
`references/solid-ddd-context.md` holds the applicability gate, per-principle detection
heuristics, and DDD checks — read it before this step. *Done when:* each principle has a
concrete finding, or the skip and its rationale are recorded.

**6. Prioritize and report.** Rank findings by impact × frequency ÷ effort into tiers, P0
(critical / blocking) through P3 (minor / litter-pickup). Write the report from
`assets/refactoring-report-template.md` and save to `docs/_refacs/<YYYYMMDD>-<slug>.md`
(create the directory; `<slug>` is a lowercase-hyphenated summary, e.g. `auth-module-cleanup`).
When the project has no tests, record in the report's risks that refactoring without coverage
is unsafe and name the critical paths to cover first. *Done when:* the saved report matches the
template and every finding sits in a tier.

**7. Verify.** Walk `checklists/analysis-checklist.md`; ship the report only once every item
passes. *Done when:* the checklist is fully satisfied.

**8. Present.** Give the user a short summary: finding counts by severity, the top 3–5
opportunities, a suggested order (quick wins before high-impact structural work), and a
complexity tier (trivial / moderate / significant) for each. Ask which refactoring to pursue.
