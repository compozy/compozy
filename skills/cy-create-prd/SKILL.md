---
name: cy-create-prd
description: Creates a Product Requirements Document through interactive brainstorming with parallel codebase and web research. Use when starting a new feature or product, building a PRD, or brainstorming requirements. Do not use for technical specifications, task breakdowns, or code implementation.
---

# Create PRD

Create a business-focused Product Requirements Document through structured brainstorming.

<HARD-GATE>
Do NOT generate the PRD document, write any file, or take any implementation action until you have presented the product design section by section and the user has approved each section. This applies to EVERY PRD regardless of perceived simplicity.
</HARD-GATE>

## Asking Questions

When this skill instructs you to ask the user a question, you MUST use your runtime's dedicated interactive question tool — the tool or function that presents a question to the user and **pauses execution until the user responds**. Do not output questions as plain assistant text and continue generating; always use the mechanism that blocks until the user has answered.

If your runtime does not provide such a tool, present the question as your complete message and stop generating. Do not answer your own question or proceed without user input.

## Anti-Pattern: "This Feature Is Too Simple For Full Brainstorming"

Every PRD goes through the full brainstorming process. A single button, a minor workflow tweak, a configuration option — all of them. "Simple" features are where unexamined business assumptions cause the most rework. The brainstorming can be brief for genuinely simple features, but you MUST ask clarifying questions and present the design for approval.

## Required Inputs

- Feature name or product idea.
- Optional: existing `_prd.md` file for update mode.

## Workflow

1. Determine the project name and working directory.
   - Derive the slug from the feature name provided by the user.
   - Use `.compozy/tasks/<slug>/` as the target directory.
   - If `_prd.md` already exists in the target directory, read it and operate in update mode.
   - If the directory does not exist, create it.
   - Create `.compozy/tasks/<slug>/adrs/` directory if it does not exist.

2. Discover context through parallel research.
   - Spawn one Agent tool call to explore the codebase for relevant patterns, existing features, and architecture.
   - Spawn a second Agent tool call to perform 3-5 web searches for market trends, competitive analysis, and user needs.
   - Merge findings from both agents before proceeding to questions.

3. Ask clarifying questions following `references/question-protocol.md`.
   - Focus exclusively on WHAT features users need, WHY it provides business value, and WHO the target users are.
   - Ask about success criteria and constraints.
   - Never ask technical implementation questions about databases, APIs, frameworks, or architecture.
   - Ask only one question per message. If a topic needs more exploration, break it into a sequence of individual questions.
   - Prefer multiple-choice questions when the options can be predetermined.
   - Complete at least one full clarification round before presenting approaches.

4. Present product approaches.
   - Offer 2-3 product approaches with trade-offs for each.
   - Lead with the recommended approach and explain why it is preferred.
   - Wait for the user to select an approach before continuing.
   - After the user selects an approach, create an ADR for this decision:
     - Read `references/adr-template.md`.
     - Determine the next ADR number by listing existing files in `.compozy/tasks/<slug>/adrs/`.
     - Fill the template: the selected approach as "Decision", rejected approaches as "Alternatives Considered" with their trade-offs, and outcomes as "Consequences". Set Status to "Accepted" and Date to today.
     - Write the ADR to `.compozy/tasks/<slug>/adrs/adr-NNN.md` (zero-padded 3-digit number, e.g., `adr-001.md`).

5. Present product design incrementally for approval.
   - After the user selects an approach, present the product design section by section.
   - Scale each section to its complexity: a few sentences if straightforward, up to 200-300 words if nuanced.
   - Present one section at a time and ask the user whether it looks right before moving to the next.
   - Sections to cover: Overview, Goals, User Stories, Core Features, User Experience, Phased Rollout, Success Metrics.
   - Be ready to revise any section based on feedback before proceeding.
   - Apply YAGNI ruthlessly: challenge every feature and remove anything the MVP does not need.
   - If the user makes a significant scope decision during refinement (e.g., including or excluding a major feature, choosing a phasing strategy), create an additional ADR following the same process as step 4.

6. Generate the PRD document.
   - Read `references/prd-template.md` and fill every section with the gathered context.
   - Include an "Architecture Decision Records" section listing all ADRs created during this session with their numbers, titles, and one-line summaries as links to the `adrs/` directory.
   - Write the completed document to `.compozy/tasks/<slug>/_prd.md`.
   - The PRD must describe user capabilities and business outcomes only.
   - No databases, APIs, code structure, frameworks, testing strategies, or architecture decisions.

## Process Flow

```dot
digraph create_prd {
    "Determine project & directory" [shape=box];
    "Discover context (codebase + web)" [shape=box];
    "Ask clarifying questions (one at a time)" [shape=box];
    "Present 2-3 product approaches" [shape=box];
    "User selects approach?" [shape=diamond];
    "Create ADR for approach decision" [shape=box];
    "Present design section by section" [shape=box];
    "User approves section?" [shape=diamond];
    "All sections approved?" [shape=diamond];
    "Generate PRD document" [shape=doublecircle];

    "Determine project & directory" -> "Discover context (codebase + web)";
    "Discover context (codebase + web)" -> "Ask clarifying questions (one at a time)";
    "Ask clarifying questions (one at a time)" -> "Present 2-3 product approaches";
    "Present 2-3 product approaches" -> "User selects approach?";
    "User selects approach?" -> "Present 2-3 product approaches" [label="no, revise"];
    "User selects approach?" -> "Create ADR for approach decision" [label="yes"];
    "Create ADR for approach decision" -> "Present design section by section";
    "Present design section by section" -> "User approves section?";
    "User approves section?" -> "Present design section by section" [label="no, revise"];
    "User approves section?" -> "All sections approved?" [label="yes"];
    "All sections approved?" -> "Present design section by section" [label="next section"];
    "All sections approved?" -> "Generate PRD document" [label="all done"];
}
```

## Error Handling

- If the user provides insufficient context to complete a section, note it in the Open Questions section rather than guessing.
- If web research tools are unavailable, proceed with codebase exploration only and note the limitation.
- If the target directory cannot be created, stop and report the filesystem error.
- If operating in update mode, preserve sections the user has not asked to change.

## Key Principles

- **One question at a time** — Do not overwhelm with multiple questions in a single message
- **Multiple choice preferred** — Easier for users to answer than open-ended when possible
- **YAGNI ruthlessly** — Challenge every feature; remove anything the MVP does not need
- **Incremental validation** — Present design section by section, get approval before moving on
- **Business focus only** — Never ask about implementation; that belongs in TechSpec
