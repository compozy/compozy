# Question Protocol

Structured brainstorming protocol for idea creation. Follow these phases and rules to guide the conversation from raw idea to structured spec.

## Phases

### 1. Problem and Pain Point

Understand the core problem or opportunity.
- What concrete problem does this feature solve for the user?
- What prompted this initiative?
- Offer 2-3 interpretations if the idea is ambiguous.

### 2. Target User and Context

Identify who benefits and when.
- Who is the primary user?
- At what point in their workflow would they use this?
- What are their current workarounds?

### 3. Scope and Priority

Define V1 boundaries.
- What is the ideal V1 size? Offer options:
  - A) Minimal MVP — solve only the main use case
  - B) Complete feature — cover the most common scenarios
  - C) Platform — extensible and configurable
- Always recommend option A or B with justification.

### 4. Ambition and Leverage

Assess the strategic value.
- How ambitious should this be? Offer options:
  - A) Quick Win — small effort, disproportionate value (ship fast, validate fast)
  - B) Strategic Bet — larger effort, potentially transformative (opens new possibilities)
  - C) Compounding Feature — gets more valuable over time (data effects, habit formation, network effects)
- Follow up: What would make this 10x more valuable instead of just incremental?

### 5. Dependencies and Integration

Map connections to existing work.
- Does this feature depend on or integrate with anything existing?
- List relevant existing features or modules as options.

### 6. Success Criteria

Define how to measure success (if not already clear from earlier answers).
- How would you know this feature is working well?
- Offer quantitative vs qualitative options.

## Rules

### Interactive Question Enforcement
- Every question MUST be asked using the runtime's dedicated interactive question tool — the one that presents the question and pauses execution until the user responds.
- Do not output questions as plain text and continue generating.
- If no such tool is available, present the question as your complete message and stop generating.

### Question Limits
- Ask only one question per message. If a topic needs deeper exploration, break it into a sequence of individual questions.
- Prefer multiple-choice questions when the options can be predetermined.
- Include a fallback option (e.g., "D) Other — describe") for flexibility.
- Wait for the user's answer before asking the next question.
- Minimum 3 questions, maximum 6.

### Progression Gates
- Must complete at least one full clarification round before proceeding to research.
- Must have clarity on problem, target user, and scope before moving forward.
- If the user's answers reveal the idea is already well-defined, skip remaining questions.

### Focus Boundaries
- Questions must focus on WHAT, WHY, and WHO.
- Never ask HOW, WHERE, or WHICH regarding technical implementation.
- Forbidden topics: databases, APIs, code structure, frameworks, testing strategies, architecture patterns, deployment infrastructure.

### YAGNI Principle
- Ruthlessly remove non-essential features during scope discussion.
- Challenge every feature: does the MVP need this?
- Defer nice-to-have features to later phases.
- Prefer smaller, well-defined scope over ambitious breadth.

### Anti-Pattern: Skipping Questions For "Obvious" Ideas
Every idea goes through the question protocol regardless of perceived simplicity. Simple ideas are where unexamined business assumptions cause the most rework. The questioning can be brief, but it must happen.
