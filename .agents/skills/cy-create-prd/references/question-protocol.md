# Question Protocol

Questioning reference for PRD creation: how to conduct the conversation from idea to decided direction. Question mechanics — interactive tool, one question per message, recommendation-led multiple-choice — are defined in SKILL.md and apply to every phase here.

## Grilling Method

Interview the user relentlessly until shared understanding: every load-bearing product decision resolved, not a fixed number of questions asked.

- Map the feature into a decision tree — which decisions exist, and which depend on which.
- Walk the tree branch by branch: ask the question that unblocks the most downstream decisions first, and resolve dependencies one at a time.
- Chase vague answers: "it depends" gets "on what?", "probably" gets pinned down. A load-bearing branch left fuzzy resurfaces as rework after the PRD ships.
- Explore before asking: when the codebase, `_idea.md`, or the research tracks already answer a question, take the answer from there and move to the next branch. User answers are for genuine unknowns — intent, priorities, trade-offs.
- Stop when the tree is resolved: every branch either has a confirmed decision or is explicitly parked in Open Questions with the user's consent.

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
- What must be true when this ships — acceptance in behavioral terms, not metrics?
- What are the known constraints (compliance, required integrations, privacy)?

### 3. Refinement

After the direction is decided and its ADR recorded (SKILL.md workflow step 4), follow up only where something is genuinely ambiguous.
- Clarify scope boundaries: what is in, and what the user rules out.
- Confirm the expected behavior of each core feature.
- Resolve any remaining open questions.

## Progression Gates

- Complete at least one full Understanding round before the direction is decided.
- Decide the direction only when every branch it depends on is resolved: purpose, constraints, and expected behavior.

## Focus Boundaries

- Questions focus on WHAT, WHY, and WHO; implementation topics (databases, APIs, code structure, frameworks, testing strategies, architecture patterns, deployment) belong to the TechSpec conversation — translate them per SKILL.md "Business Focus".
