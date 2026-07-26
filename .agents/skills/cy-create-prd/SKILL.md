---
name: cy-create-prd
description: Creates a Product Requirements Document plus its companion user-story catalog (_user_stories.md) through interactive brainstorming with parallel codebase and web research. Use when starting a new feature or product, building a PRD, brainstorming requirements, or cataloging user stories and edge cases. Do not use for technical specifications, task breakdowns, or code implementation.
argument-hint: "[feature-name-or-idea] [idea-file]"
---

# Create PRD

Create a business-focused Product Requirements Document and its companion user-story catalog through structured brainstorming.

Both documents are written for the LLM agents that consume them downstream (`cy-create-techspec`, `cy-create-tasks`, review rounds). Their job is to supply business rules, domain behavior, and product intent. KPIs, success metrics, timelines, and rollout phases have no consumer in this pipeline — leave them out.

<HARD-GATE>
- Research before questions: every PRD MUST be enriched with codebase and market context.
- Questions before writing: the user MUST shape the PRD by answering clarifying questions — for every PRD, however simple. "Simple" features are where unexamined business assumptions cause the most rework; brief brainstorming is fine, skipped brainstorming is not.
- Decide, then write: once the questions are answered and the ADRs are recorded, write the files directly. The user reviews the generated files and requests changes afterward — no approach menus, no draft-approval loops.
</HARD-GATE>

## Full Scope, One PRD

Capture the complete scope the user wants in this single PRD, however large it grows. Agents run long and `cy-create-tasks` decomposes the work later, so document size is never a reason to trim, defer, or stage anything.

- A capability leaves the PRD only when the user decides against it — record that in Non-Goals.
- YAGNI applies to invention: challenge features the user never asked for; keep every one they did.
- When the user adds scope mid-conversation, fold it in and keep going.

## Asking Questions

Ask every question through the runtime's dedicated interactive question tool — the mechanism that presents a question and pauses execution until the user responds. If the runtime has no such tool, present the question as the complete message and stop generating; never answer a question on the user's behalf.

- One question per message: exactly one question mark, then stop. A topic that needs more exploration gets its follow-up in the next message, after the user answers.
- Lead with a recommendation: state which option you would pick and why in one line, so the user reacts to a position instead of facing a blank menu.
- Multiple-choice whenever the options can be predetermined: labeled options (A, B, C) with your recommendation first, plus a fallback ("D) Other — describe"). Open-ended only when the answer space is genuinely unbounded.
- For features with many dimensions, ask about one dimension at a time ("Which aspect of team collaboration matters most first? A) Shared workspaces B) Real-time presence C) Permission controls D) Activity feeds").

## Business Focus

The PRD owns WHAT users need, WHY it provides value, and WHO the users are; HOW belongs to the TechSpec. When the feature name sounds technical ("webhook notifications", "CSV export", "API rate limiting"), translate it into the user-experience question behind it:

- WRONG: "Should we use WebSockets or polling for notifications?" (implementation)
- RIGHT: "Which events should trigger a notification to the user?" (user need)

## Required Inputs

- Feature name or product idea.
- Optional: existing `_idea.md` file as primary input for context.
- Optional: existing `_prd.md` file for update mode.

## Workflow

Track each step as a task in the runtime's task tracker when one is available, and complete the steps in order.

1. Determine the project and working directory.
   - Derive the slug from the feature name; the target directory is `.compozy/tasks/<slug>/`.
   - Create the directory and its `adrs/` subdirectory if missing.
   - If `_idea.md` exists there, read it as primary context.
   - If `_prd.md` exists, read it and operate in update mode.

2. Discover context through two parallel research tracks. Both MUST finish before any question is asked; run them in parallel (e.g., two Agent tool calls).
   - Track A — Codebase: search for files, patterns, data models, and integration points related to the request; summarize in 3-5 bullets.
   - Track B — Market: perform 3-5 web searches on trends, competing products, and user expectations; summarize in 3-5 bullets. If web search tools are unavailable, note the limitation and proceed with Track A only.
   - Present the merged findings from both tracks to the user before moving to questions.

3. Grill the requirements.
   - Read `references/question-protocol.md` and apply its Grilling Method through its phases, resolving the load-bearing product decisions branch by branch.
   - Done when every branch that shapes the PRD is resolved or explicitly parked for Open Questions — the question count is an output of the decision tree, not a budget.

4. Decide the product approach and record ADRs.
   - Choose the strongest direction yourself from the answers and research.
   - Read `references/adr-template.md`, determine the next number from the files in `.compozy/tasks/<slug>/adrs/`, fill the template (chosen direction as Decision, weighed alternatives with trade-offs as Alternatives Considered, outcomes as Consequences; Status "Accepted", Date today), and write `adrs/adr-NNN.md` (zero-padded 3-digit number).
   - Record any additional significant scope decision that surfaced during clarification as its own ADR.

5. Write the user-story catalog.
   - Read `references/user-stories-template.md` and write `.compozy/tasks/<slug>/_user_stories.md`.
   - Cover every persona — secondary ones included — and every core feature.
   - Run the template's edge-case sweep against every story.
   - Done when every core feature has stories, every story has verifiable acceptance criteria plus edge cases with expected behavior, and every edge-case class has been probed against every story.

6. Write the PRD.
   - Read `references/prd-template.md` and fill every section with the decided direction and confirmed answers; the template carries the per-section rules.
   - List every ADR from this session in the Architecture Decision Records section.
   - Prefer active voice and definite, specific language; every sentence earns its place. Language: English.
   - Write `.compozy/tasks/<slug>/_prd.md`.

7. Hand off.
   - Confirm both file paths to the user and invite change requests directly on the generated files.
   - Point to `cy-create-techspec` as the next step.

## Error Handling

- Insufficient context for a section: note it in Open Questions rather than guessing.
- Web research tools unavailable: proceed with codebase findings and state the limitation.
- Target directory cannot be created: stop and report the filesystem error.
- Update mode: preserve sections the user has not asked to change, and mirror any story change into `_user_stories.md` so the catalog and the PRD stay in sync.
