# Idea Template

Use this template to structure every idea in `.compozy/tasks/<slug>/_idea.md`. Fill each section based on research and brainstorming outcomes. Leave placeholder guidance in sections where information is insufficient and note them in Open Questions.

## Overview

High-level overview of the feature idea. Describe:

- What problem it solves
- Who it is for
- Why it is valuable
- How ambitious the V1 should be

## Problem

The real problem the user or business faces:

- 2-4 paragraphs with concrete scenarios and data
- Why the current solution is insufficient
- Include market data as a "### Market Data" subsection when available from research

## Core Features

Main features grouped by priority:

| #   | Feature | Priority               | Description                                                 |
| --- | ------- | ---------------------- | ----------------------------------------------------------- |
| F1  | {Name}  | {Critical/High/Medium} | {Concise description of the feature with expected behavior} |
| F2  | {Name}  | {Critical/High/Medium} | {Description}                                               |

Rules:

- Number features with a 2-letter prefix (e.g., F1, F2)
- Order by priority (Critical > High > Medium)
- Each feature described in 1-2 lines with concrete behavior
- Minimum 3 features, maximum 10

## KPIs

Quantifiable measures of success from business analysis:

| KPI                 | Target                    | How to Measure                |
| ------------------- | ------------------------- | ----------------------------- |
| {Observable metric} | {Numeric value with unit} | {Concrete measurement method} |

Rules:

- Minimum 3 KPIs, maximum 6
- Targets must be numeric and measurable (e.g., "> 30%", "< 200ms", "-80%")
- "How to Measure" must be concrete and implementable

## Feature Assessment

Score from the business analysis phase:

| Criteria            | Question                                            | Score   |
| ------------------- | --------------------------------------------------- | ------- |
| **Impact**          | How much more valuable does this make the product?  | {score} |
| **Reach**           | What % of users would this affect?                  | {score} |
| **Frequency**       | How often would users encounter this value?         | {score} |
| **Differentiation** | Does this set us apart or just match competitors?   | {score} |
| **Defensibility**   | Is this easy to copy or does it compound over time? | {score} |
| **Feasibility**     | Can we actually build this?                         | {score} |

Leverage type: {Quick Win / Strategic Bet / Compounding Feature}

## Council Insights

Key findings from the multi-advisor debate:

- **Recommended approach:** {summary of the council recommendation}
- **Key trade-offs:** {list of trade-offs surfaced}
- **Risks identified:** {list of risks and mitigations}
- **Stretch goal (V2+):** {optional more ambitious version for later}

## Out of Scope (V1)

Explicitly excluded features and boundaries:

- **{Excluded feature}** — {short justification for why it is out of V1 scope}

Rules:

- Minimum 3 exclusions
- Each item must have a justification
- Helps prevent scope creep and align expectations

## Architecture Decision Records

ADRs documenting key decisions made during idea creation:

- [ADR-NNN: Title](adrs/adr-NNN.md) — One-line summary of the decision

## Open Questions

Remaining items that need clarification:

- Unclear requirements
- Edge cases requiring stakeholder input
- Dependencies on decisions not yet made

## Optional Sections

Include these between mandatory sections when the content justifies it:

### Summary / Differentiator

Short paragraph describing the competitive angle of the proposed solution. Use when the feature has a clear differentiator vs the market.

### Integration with Existing Features

| Integration Point         | How                            |
| ------------------------- | ------------------------------ |
| {Existing feature/module} | {How the new feature connects} |

Use when the feature modifies or extends already implemented features.

### Sub-Features

- **{Name}** — {Brief description of the sub-scope}

Use when the feature is large enough to be split into multiple ideas or PRDs.

### Cost Estimate

| Type       | Volume            | Estimated Cost        |
| ---------- | ----------------- | --------------------- |
| {Resource} | {Expected volume} | {Monthly/annual cost} |

Use for features with relevant operational costs (paid APIs, cloud storage, etc.).
