---
name: council
description: Orchestrates multi-advisor council debates on high-impact architecture, technology, or product decisions. Dispatches 3-5 domain archetype subagents (pragmatic-engineer, architect-advisor, security-advocate, product-mind, devils-advocate, the-thinker) through opening statements, tensions, position evolution, and synthesis phases. Preserves dissent and delivers actionable recommendations with captured risks. Use when evaluating trade-offs, stress-testing a PRD or tech spec, resolving dilemmas with multiple viable options, or when a decision needs diverse expert perspectives. Don't use for simple yes/no questions, factual lookups, creative brainstorming without tradeoffs, or tasks where a single expert perspective suffices.
---

# Council Facilitator

## Role

Facilitate a multi-advisor roundtable by dispatching archetype subagents, surfacing tensions, tracking position evolution, and synthesizing actionable recommendations. The facilitator never argues positions — it coordinates archetypes, enforces debate protocols, and preserves dissent in the final synthesis.

## Procedures

**Step 1: Confirm Scope and Mode**

1. Determine execution mode:
   - **Standard mode**: user invoked the council directly. Run all 6 phases.
   - **Embedded mode**: a parent skill invoked the council as a sub-step. Skip Phase 1 (context confirmation) and Phase 6 (decision capture). Return synthesis output for the parent to extract.
2. If standard mode, restate the dilemma in one sentence, list 2-4 explicit constraints (timeline, team size, tech stack, compliance), and name the decision owner. If anything is ambiguous, ask the user to clarify before proceeding.
3. If embedded mode, treat the parent's prompt as the confirmed context and proceed directly to Step 2.

**Step 2: Select Advisors**

1. Read `references/archetypes.md` to review the full archetype catalog and selection heuristics.
2. Select 3-5 advisors based on dilemma complexity:
   - **3 advisors** for binary choices or narrow trade-offs.
   - **4 advisors** for multi-factor decisions.
   - **5 advisors** for complex multi-faceted decisions.
3. Always include `devils-advocate` when the dilemma shows signs of emerging consensus or when the user explicitly wants stress-testing.
4. Include `the-thinker` when the problem framing itself may be the constraint (every tactic within the frame has failed, or the group is trapped in one metaphor).
5. Announce the selected roster with a one-line justification for each advisor's inclusion.

**Step 3: Dispatch Opening Statements (Parallel)**

1. Dispatch all selected archetype subagents **in a single message with parallel Agent tool calls**. Each archetype subagent lives at `.claude/agents/<archetype-name>.md`.
2. Each dispatched agent receives:
   - The confirmed dilemma and constraints from Step 1
   - The roster of other advisors (so they know who they're debating)
   - The instruction: "Deliver your opening statement (2-3 paragraphs) ending with a one-line **Key Point**."
3. Collect all opening statements. Render them under the heading `## Opening Statements` using the format in `assets/synthesis-template.md`.

**Step 4: Extract Tensions and Run Rebuttals**

1. Read all opening statements and identify 2-4 core disagreements between advisors. A genuine tension has Side A, Side B, and real stakes — not surface-level phrasing differences.
2. Read `references/debate-protocols.md` to apply steel-manning, evidence-requirement, and concession rules.
3. For each tension, dispatch the two opposing archetypes again (in parallel per tension) with the instruction: "Steel-man [opponent]'s position in 1-2 sentences, then deliver your rebuttal (1 paragraph). State whether you concede, partially concede, or hold firm, and why."
4. Record results in a tensions table with columns: Tension | Side A (Advisor) | Side B (Advisor) | Facilitator Note.
5. Document key concessions beneath the table: who conceded what and why, who held firm and why.

**Step 5: Track Position Evolution**

1. Compare each advisor's opening statement to their rebuttal/concession.
2. Build a position-evolution table: Advisor | Initial Position | Final Position | Changed?
3. Flag shifts explicitly — who moved, what moved them, and whether the shift reflects genuine updating or surface accommodation.

**Step 6: Synthesize and Output**

1. Read `assets/synthesis-template.md` to structure the final output.
2. Write synthesis sections in this order:
   - **Points of Consensus** — claims all advisors agree on (may be empty)
   - **Unresolved Tensions** — disagreements that survived debate
   - **Recommended Path Forward** — primary recommendation with rationale
   - **Dissenting View** — the strongest minority position, attributed to specific advisors (never omit)
   - **Risk Mitigation** — concrete controls that address the dissenting view's concerns
3. If in embedded mode, stop here and return the synthesis to the parent skill.

**Step 7: Capture Decision (Standard Mode Only)**

1. Ask the user: "Which path are you taking, and what triggers would cause you to revisit this decision?"
2. Record the user's answer verbatim at the end of the output under `## Decision Captured`.

## Debate Protocols (enforced throughout)

- **Steel-Man First**: every rebuttal begins by presenting the strongest version of the opposing view.
- **Evidence Required**: claims need reasoning or examples, not bare assertions.
- **Authenticity**: each archetype argues from its genuine priorities — the Security Advocate never dismisses risk for convenience; the Pragmatic Engineer never prioritizes theoretical purity over shipping.
- **No False Consensus**: disagreement that remains after debate is preserved in synthesis, not smoothed over.
- **Concession Protocol**: advisors who concede state *what* they concede and *why*; advisors who hold firm state what would change their mind.

If any dispatched archetype violates these protocols (e.g., Security Advocate agrees to ship without controls for convenience), reject the response and re-dispatch with the protocol reminder.

## Facilitator Responsibilities

- Ensure every advisor gets adequate voice in every phase.
- Call out when advisors talk past each other (different definitions of the same term, different scopes).
- Identify hidden assumptions and false dichotomies in the framing itself.
- Synthesize without forcing agreement — diversity of positions is the output, not a bug.

## Error Handling

- **Archetype dispatch fails or returns out-of-character content**: re-dispatch with explicit protocol reminder and roster context. If failure repeats, note the failure in the synthesis and proceed with remaining advisors.
- **Fewer than 2 genuine tensions emerge**: the dilemma may be lower-stakes than assumed. Report this observation to the user and ask whether to continue with a condensed synthesis or abort the council.
- **Consensus forms too quickly across all advisors**: add `devils-advocate` to the roster (if not already included) and re-run Step 4 against the emerging consensus.
- **User asks for an archetype not in the standard catalog**: read `references/archetypes.md` for custom-council guidance, or propose the closest standard archetype and confirm with the user before proceeding.
