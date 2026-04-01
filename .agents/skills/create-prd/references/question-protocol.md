# Question Protocol

Structured brainstorming protocol for PRD creation. Follow these phases and rules to guide the conversation from idea to document.

## Phases

### 1. Discovery

Gather initial context about the idea or problem space.
- What is the core problem or opportunity?
- Who are the affected users?
- What prompted this initiative?

### 2. Understanding

Deepen knowledge of requirements and constraints.
- WHAT specific features do users need?
- WHY does this provide business value?
- WHO are the target users and what are their current workflows?
- What are the success criteria?
- What are the known constraints (timeline, budget, compliance)?

### 3. Options

Present product approaches for the user to evaluate.
- Offer 2-3 distinct approaches with clear trade-offs.
- Lead with the recommended approach and explain why.
- Each approach should differ meaningfully in scope, phasing, or strategy.
- Wait for the user to select before proceeding.

### 4. Refinement

Refine the selected approach with targeted follow-ups.
- Clarify scope boundaries for the chosen approach.
- Confirm phasing and priority of features.
- Validate success criteria and metrics.
- Resolve any remaining open questions.

### 4b. Incremental Design Validation

Present the product design section by section for user approval.
- Scale each section to its complexity: brief for straightforward topics, detailed for nuanced ones.
- Present one section at a time; ask whether it looks right before moving on.
- Apply YAGNI: challenge every feature against MVP necessity.
- Be ready to revise any section before proceeding to the next.

### 5. Creation

Generate the PRD document using the gathered context.
- Read and fill the PRD template.
- Every section should reflect confirmed decisions.
- Unresolved items go into Open Questions.

## Rules

### Interactive Question Enforcement
- Every question MUST be asked using the runtime's dedicated interactive question tool — the one that presents the question and pauses execution until the user responds.
- Do not output questions as plain text and continue generating.
- If no such tool is available, present the question as your complete message and stop generating.

### Question Limits
- Ask only one question per message. If a topic needs deeper exploration, break it into a sequence of individual questions.
- Prefer multiple-choice questions when the options can be predetermined.
- Wait for the user's answer before asking the next question.

### Progression Gates
- Must complete at least one full Understanding round before presenting Options.
- Must have clarity on purpose, constraints, and success criteria before presenting approaches.
- Must have user approval of an approach before entering Refinement.

### Focus Boundaries
- Questions must focus on WHAT, WHY, and WHO.
- Never ask HOW, WHERE, or WHICH regarding technical implementation.
- Forbidden topics: databases, APIs, code structure, frameworks, testing strategies, architecture patterns, deployment infrastructure.

### YAGNI Principle
- Ruthlessly remove non-essential features during refinement.
- Challenge every feature: does the MVP need this?
- Defer nice-to-have features to later phases.
- Prefer smaller, well-defined scope over ambitious breadth.

### Anti-Pattern: Skipping Brainstorming For "Simple" Features
Every PRD goes through the full question protocol regardless of perceived simplicity. Simple features are where unexamined business assumptions cause the most rework. The brainstorming can be brief, but it must happen.
