Run a comprehensive refactoring analysis of the codebase using parallel subagents, each covering a group of packages.

All documents must be saved to `.compozy/tasks/$ARGUMENTS/` (create the directory if it does not exist).

## Analysis Scope

Each subagent MUST use the refactoring-analysis skill and cover ALL of the following aspects (not just code smells):

1. **Code smells** — long functions, duplication, conditional complexity, dead code
2. **Architectural boundaries** — packages doing too much that should be split into separate packages (not just file splits), mixed responsibilities within a single package
3. **Coupling analysis** — afferent/efferent coupling, excessive inter-package coupling, circular dependencies
4. **Dependency inversion** — high-level packages importing concrete implementations, opportunities for inversion via interfaces
5. **Domain modeling** — domain concepts that lack their own package but should have one (e.g., a concept scattered across 3+ packages), domain cohesion
6. **SOLID violations** — SRP at the package level (not just file level), ISP for large interfaces, OCP for extensibility
7. **DRY violations** — cross-package duplication of types, functions, patterns

For each finding, the subagent MUST recommend the action type:

- **(A) File-level split** — reorganize files within the same package
- **(B) Package-level split** — create a new package from part of an existing one
- **(C) Extraction** — move to an existing or new shared package
- **(D) Inline fix** — refactoring within the file

## Package Groups

Split the packages into 4-5 logical groups for the subagents. Launch all of them in parallel.

## Subagent Output

Each subagent creates its report in `.compozy/tasks/$ARGUMENTS/` named `<YYYYMMDD>-<slug>.md`.

## Consolidated Summary Document

After all subagents complete, create a final document at `.compozy/tasks/$ARGUMENTS/<YYYYMMDD>-summary.md` that:

- Summarizes ALL findings with severity (P0-P3) and action type (A/B/C/D)
- Has a dedicated **"Architectural Recommendations"** section separate from code smells, covering:
  - Packages that should be **created** (new package for a domain concept)
  - Packages that should be **split** (existing package doing too much)
  - Packages that should be **merged** (redundant packages)
  - Domain concepts mapped to packages (current state vs. proposed)
- Includes a **current dependency diagram** (mermaid)
- Includes a **proposed dependency diagram** (mermaid) showing the state after recommendations
- Lists the top 10 opportunities ranked by impact
- Proposes a phased execution roadmap

## Verification

After creating the final document, launch a verification subagent that:

- Reads all generated documents
- Verifies math (totals, severities)
- Spot-checks 10+ claims against the actual codebase (line counts, function locations)
- Identifies contradictions or overlaps between reports
- Reports any errors found so they can be corrected in the summary
