# Debate Protocols

These protocols govern every phase of a council session. Enforce them on every dispatched archetype response. Reject and re-dispatch responses that violate them.

## Steel-Man First

Every rebuttal, concession, or challenge begins by presenting the **strongest plausible version** of the opposing argument in 1-2 sentences.

- **Pass**: "The Architect's strongest case is that today's coupling will cost 3+ weeks on every migration for the next 2 years, which dwarfs the one-quarter split cost."
- **Fail**: "The Architect wants to do a big rewrite." (strawman — dismissive, weakens the opposing case before engaging it)

If an advisor cannot steel-man the opposing view, they are not qualified to critique it.

## Evidence Required

Claims need reasoning, concrete examples, or explicit trade-off analysis — not bare assertions.

- **Pass**: "Token revocation is broken for JWT because the server cannot invalidate a token it didn't store; compromise requires waiting for TTL expiry."
- **Fail**: "JWT is insecure."

If a response contains only assertions, re-dispatch asking for the specific mechanism, scenario, or evidence.

## Authenticity

Each archetype argues from its **genuine priorities** — documented in `archetypes.md`. They do not drift toward whichever position sounds diplomatic.

- **Pass (Security Advocate)**: "I'll concede the theoretical risk is small, but compliance requires audit trails regardless — that's not optional."
- **Fail (Security Advocate)**: "Sure, we can ship without the audit trail this quarter and add it later."

Authenticity violations usually look like:
- Security Advocate agreeing to skip controls for velocity
- Pragmatic Engineer championing a rewrite over incremental delivery
- Architect accepting "we'll refactor it later" without a plan
- Product Mind approving work with no value hypothesis
- Devil's Advocate attacking strawmen instead of strongest forms
- The Thinker offering decorative analogies that don't predict new moves

Reject authenticity violations and re-dispatch with a reminder of the archetype's priorities.

## Concession Protocol

Advisors who concede must state:
- **What** they concede (the specific claim or position)
- **Why** they concede (the argument or evidence that moved them)

Advisors who hold firm must state:
- **What would change their mind** (a concrete scenario, evidence, or mitigation)

- **Pass**: "I concede the split can ship incrementally, because the Architect's phased-extraction proposal addresses the 'big-bang rewrite' concern I had. I still hold firm on requiring the oncall runbook before merge."
- **Fail**: "Good points all around." (no concession content, no held-firm content, no signal of genuine engagement)

## No False Consensus

Disagreement that survives debate is **preserved** in synthesis. Never smooth over tensions for a cleaner recommendation.

- **Pass**: "The council reached no consensus on timing; Security Advocate holds that controls must ship before the feature, while Product Mind maintains that a post-launch controls sprint is acceptable. This dissent is captured in the Dissenting View section."
- **Fail**: "After discussion, advisors agreed the feature should ship with security controls eventually." (papers over the timing disagreement, which is the load-bearing tension)

## Productive Conflict

Disagreement is a **feature**, not a bug. The council's value comes from exploring tensions, not reaching unanimous agreement. If all advisors agree after Phase 3, that is a signal of:
1. A lower-stakes dilemma than assumed, or
2. Missing perspective (add `devils-advocate` if not present), or
3. Archetypes drifting out of character (re-dispatch with authenticity reminder)

## Avoid Talking Past Each Other

Two advisors can appear to disagree while defining the same term differently, or scoping the decision differently. When this happens, the facilitator **names it explicitly** in the Tensions table's Facilitator Note column and asks both advisors to align on definition/scope before the rebuttal round continues.

Example Facilitator Note: "Both advisors use 'split' but the Architect means 'service-level boundary split' while the Pragmatic Engineer means 'module-level package split'. Re-aligning scope."
