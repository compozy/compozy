# TechSpec Quality Markers

When the user opts into peer review, a TechSpec is "ready for peer review" only when all six markers below are present. These correlate with high-signal Opus output (tight blockers, actionable nits) versus shallow noise (Opus rediscovering missing sections or vague design). Markers align with the canonical template in `.agents/skills/cy-create-techspec/references/techspec-template.md`.

If any marker is missing, abort the requested peer review and ask the user to amend the spec first. Opus review on incomplete specs wastes credit and produces noise.

## Marker 1: Scope Boundary

The `Executive Summary` (or equivalent opening section) explicitly states what is in scope, what is deferred, and what is out of scope for this TechSpec. A reader can tell where implementation ends without reading the full document.

Example:

> "MVP boundary: recovery orchestration, agentic remediation, and workspace config ship in this task. Post-MVP consumer-triggered automatic runs and multi-attempt budgets remain follow-up work unless explicitly pulled into scope later."

## Marker 2: Component Boundaries

The `System Architecture` section (including `Component Overview` when used as a subsection) names each component, its responsibility, and its boundary. New packages or modules are named explicitly. Cross-component dependencies are visible — not buried in prose elsewhere.

## Marker 3: Concrete Interface Signatures

Critical interfaces appear as code blocks with final method/function signatures — not described only in prose. Error handling conventions are stated or visible in the signatures. Every signature the implementer will code against is present.

## Marker 4: Data Model Rationale

The `Data Models` section (or equivalent under `Implementation Design`) lists entities, fields, storage shapes, and the purpose of each. When schema or config keys change, the spec states what changes and why — not just "add a column."

## Marker 5: Testing Strategy

The `Testing Approach` section covers unit and integration testing with enough detail to implement tests: key scenarios, mock boundaries, environment dependencies, and edge cases tied to the risks described in the spec. A vague "we will add tests" paragraph does not satisfy this marker.

## Marker 6: Build Order and Dependencies

The `Development Sequencing` section (including `Build Order` when used as a subsection) lists an ordered implementation sequence with blocking dependencies. The reader can tell which step must land before the next and which external or team deliverables block progress.
