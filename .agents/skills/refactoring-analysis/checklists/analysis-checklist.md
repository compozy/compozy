# Refactoring Analysis Checklist

Verify each item before finalizing the report.

---

## Scope & Context

- [ ] Analysis target is clearly defined (directory, module, or feature area)
- [ ] Language and paradigm identified (OOP, functional, mixed)
- [ ] Project conventions understood (naming, structure, patterns)
- [ ] Test coverage status assessed and noted in report

## Smell Detection — Completeness

- [ ] Scanned for **Bloaters**: Long Functions, Large Classes, Long Parameter Lists,
      Data Clumps, Primitive Obsession
- [ ] Scanned for **Change Preventers**: Divergent Change, Shotgun Surgery
- [ ] Scanned for **Dispensables**: Duplicated Code, Dead Code, Speculative Generality,
      Lazy Elements, Comments as Deodorant
- [ ] Scanned for **Couplers**: Feature Envy, Insider Trading, Message Chains, Middle Man
- [ ] Scanned for **Conditional Complexity**: Nested Conditionals, Repeated Switches,
      Complex Booleans
- [ ] Scanned for **DRY Violations**: Magic Numbers, Copy-Paste Variations, Repeated
      Parameter Groups

## Refactoring Mapping

- [ ] Each smell has a mapped refactoring technique from the catalog
- [ ] Before/after code sketches provided for P0 and P1 findings
- [ ] Technique names use Fowler's canonical names

## Coupling & Cohesion

- [ ] Module dependency structure analyzed
- [ ] High afferent coupling modules identified (risky to change)
- [ ] High efferent coupling modules identified (fragile)
- [ ] Circular dependencies checked
- [ ] Cohesion assessed (mixed responsibilities flagged)

## DRY Analysis

- [ ] Near-duplicate code blocks identified
- [ ] Magic numbers and strings cataloged
- [ ] Repeated parameter patterns found
- [ ] Extraction strategies proposed

## SOLID (If Applicable)

- [ ] Confirmed project uses DDD/hexagonal/clean architecture before analyzing
- [ ] Or: noted "SOLID analysis skipped" with rationale
- [ ] Each principle evaluated with concrete findings (not generic advice)

## Prioritization

- [ ] All findings have severity: critical / high / medium / low
- [ ] Priority considers impact × frequency ÷ effort
- [ ] Quick wins identified (low effort, high clarity improvement)
- [ ] Suggested refactoring order accounts for dependencies between changes

## Report Quality

- [ ] Report follows template from `assets/refactoring-report-template.md`
- [ ] Executive summary is concise (2-4 sentences) and leads with biggest opportunity
- [ ] Top opportunities table filled with 3-5 entries
- [ ] File paths are exact (file:line format)
- [ ] Code snippets are real (not fabricated), simplified to essential lines
- [ ] Rationale explains "why" not just "what"
- [ ] Risks and caveats section acknowledges ambiguity and limitations
- [ ] Saved to `docs/_refacs/<YYYYMMDD>-<slug>.md`

## Integrity

- [ ] No fabricated findings — every smell references real code
- [ ] Ambiguous cases flagged as "potential" with context
- [ ] Framework-specific patterns not falsely flagged (e.g., Redux boilerplate is expected)
- [ ] Intentional design choices acknowledged (e.g., Strategy pattern is not Feature Envy)
