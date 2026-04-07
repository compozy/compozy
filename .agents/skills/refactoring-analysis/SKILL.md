---
name: refactoring-analysis
description: >
  Analyzes codebases to identify refactoring opportunities based on Martin Fowler's catalog
  of code smells and refactoring techniques. Detects duplicated code, high coupling, complex
  conditionals, primitive obsession, long functions, and other structural issues. Produces
  a structured refactoring report with prioritized findings saved to docs/_refacs/.
  Use when auditing code quality, preparing for a refactoring sprint, or reviewing
  architectural health. Don't use for style/formatting issues, performance optimization,
  or security audits.
---

# Refactoring Analysis

Perform a systematic analysis of a codebase to identify refactoring opportunities based on
Martin Fowler's "Refactoring: Improving the Design of Existing Code" (2nd Edition).
Produce a prioritized report with actionable findings.

## Procedures

**Step 1: Scope the Analysis**

1. Determine the analysis target — a specific directory, module, feature area, or the
   entire project. If the user did not specify, ask which area to focus on.
2. Identify the project's language and paradigm (OOP, functional, mixed) to calibrate
   which smells and techniques are applicable.
3. If the project follows domain-driven design (DDD) or hexagonal architecture, read
   `references/solid-ddd-context.md` for additional SOLID-specific analysis criteria.

**Step 2: Explore the Codebase**

1. Map the directory structure and identify key modules, entry points, and shared utilities.
2. Read the most critical files (entry points, core business logic, shared modules).
3. Identify the project's conventions: naming, file organization, test patterns, dependency
   injection approach.

**Step 3: Detect Code Smells**

Read `references/code-smells-catalog.md` for the full catalog of detectable smells.

Systematically scan for the following smell categories, in priority order:

1. **Bloaters** — Long Functions (>15 lines of logic), Large Classes/Modules (>300 lines),
   Long Parameter Lists (>3 params), Data Clumps, Primitive Obsession.
2. **Change Preventers** — Divergent Change (one module changed for multiple unrelated
   reasons), Shotgun Surgery (one change touches 5+ files).
3. **Dispensables** — Duplicated Code, Dead Code, Speculative Generality, Lazy Elements,
   Comments as Deodorant.
4. **Couplers** — Feature Envy, Insider Trading (excessive data sharing between modules),
   Message Chains (>2 levels deep), Middle Man (>50% delegation).
5. **Conditional Complexity** — Nested Conditionals (>2 levels), Repeated Switches,
   Missing Guard Clauses, Complex Boolean Expressions.
6. **DRY Violations** — Near-identical code blocks, copy-pasted logic with minor
   variations, repeated parameter groups, duplicated constants or magic numbers.

For each detected smell:
- Record the exact file path and line range
- Classify the smell type (from the catalog)
- Assess severity: `critical` | `high` | `medium` | `low`
- Note the impact on maintainability, readability, or change cost

**Step 4: Map Refactoring Opportunities**

Read `references/refactoring-techniques.md` for the full technique catalog.

For each detected smell, identify the recommended refactoring technique(s):

| Smell | Primary Technique |
|-------|-------------------|
| Long Function | Extract Function, Decompose Conditional |
| Duplicated Code | Extract Function, Pull Up Method |
| Long Parameter List | Introduce Parameter Object, Preserve Whole Object |
| Feature Envy | Move Function |
| Data Clumps | Extract Class, Introduce Parameter Object |
| Primitive Obsession | Replace Primitive with Object |
| Large Class/Module | Extract Class, Extract Module |
| Repeated Switches | Replace Conditional with Polymorphism |
| Message Chains | Hide Delegate, Extract Function |
| Nested Conditionals | Replace with Guard Clauses |
| Dead Code | Remove Dead Code |
| Magic Numbers/Strings | Extract Constant |
| Mutable Shared State | Encapsulate Variable, Split Variable |
| Imperative Loops | Replace Loop with Pipeline (map/filter/reduce) |

For each opportunity, draft a concrete before/after code sketch showing the transformation.

**Step 5: Assess Coupling and Cohesion**

1. Identify modules with high afferent coupling (many dependents — risky to change).
2. Identify modules with high efferent coupling (many dependencies — fragile).
3. Flag circular dependencies between modules.
4. Assess cohesion — modules that mix unrelated responsibilities are candidates for
   Extract Class or Split Phase.

**Step 6: Evaluate DRY Opportunities**

1. Search for near-duplicate code blocks (>5 lines of similar structure).
2. Identify repeated constant values (magic numbers, repeated string literals).
3. Find repeated parameter patterns across function signatures.
4. Look for copy-pasted logic with minor variations that could be parameterized.
5. Propose extraction strategies: shared functions, constants files, parameter objects.

**Step 7: SOLID Analysis (Domain Projects Only)**

<GATE>
Only perform this step if the project uses domain-driven design (DDD), hexagonal/clean
architecture, or explicitly models a complex business domain. SOLID principles have the
most impact in domain-rich, object-oriented codebases. For simple CRUD apps, utility
libraries, or purely functional codebases, skip this step and note in the report that
SOLID analysis was not applicable.
</GATE>

Read `references/solid-ddd-context.md` for detailed guidance.

Evaluate against SOLID principles with domain-project focus:
- **SRP**: Classes/modules with multiple reasons to change
- **OCP**: Areas requiring modification (not extension) for new variants
- **LSP**: Subclasses that override to throw or no-op (Refused Bequest smell)
- **ISP**: Interfaces forcing implementers to stub unused methods
- **DIP**: High-level modules importing low-level implementations directly

**Step 8: Prioritize and Generate Report**

1. Read `assets/refactoring-report-template.md` for the output template.
2. Rank all findings by a priority score combining:
   - **Impact**: How much does this hurt readability, maintainability, or change cost?
   - **Frequency**: How often is this pattern encountered in the codebase?
   - **Effort**: How much work to refactor? (low effort + high impact = do first)
3. Group findings into priority tiers:
   - **P0 — Critical**: Blocking future development, causing bugs, or high coupling
   - **P1 — High**: Significant maintenance burden, frequent pain point
   - **P2 — Medium**: Noticeable but manageable, worth addressing opportunistically
   - **P3 — Low**: Minor improvements, cosmetic, litter-pickup candidates
4. Generate the report following the template and save to:
   `docs/_refacs/<YYYYMMDD>-<slug>.md`
   where `<slug>` is a lowercase-hyphenated summary (e.g., `auth-module-cleanup`).
5. Create the `docs/_refacs/` directory if it does not exist.

**Step 9: Present Summary**

1. Present a brief executive summary to the user with:
   - Total findings count by severity
   - Top 3-5 highest-impact opportunities
   - Suggested refactoring order (quick wins first, then high-impact)
   - Estimated complexity tier for each (trivial / moderate / significant)
2. Ask the user if they want to proceed with any specific refactoring.

## Error Handling

* If the analysis target is too broad (>50 files), ask the user to narrow scope or
  confirm they want a high-level scan with sampling.
* If the project has no tests, warn that refactoring without test coverage is risky
  and recommend adding tests for critical paths before refactoring.
* If the project uses an unfamiliar framework or pattern, note this limitation in
  the report rather than guessing.
* If a smell is ambiguous (could be intentional design), flag it as "potential" and
  note the context that might justify the current structure.
