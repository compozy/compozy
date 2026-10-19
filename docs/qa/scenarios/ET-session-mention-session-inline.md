---
id: ET-session-mention-session-inline
area: ET
title: Session mention to a tools-disabled or policy-denied session uses the inline or unavailable form
persona: Théo
journey:
expected: When the receiving session has tools disabled and the operator sends the prompt, the references block carries each mentioned session's recent history inside `<recent_history>` within the 20 message / 1,500 / 8,000 / 16,000 character budgets with `<`, `>`, `&` escaped; when compozy__session_history is denied by policy or a narrowed spawn policy, or another agent sends the prompt to a tools-disabled session, the block lists references only with "Session history is not available to you; ask the operator if you need it." and no history is read.
entry_points: web session window composer; compozy session prompt; compozy__session_prompt; compozy session events --json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-mention-session
---

US-006.AC-3/AC-5, US-006.EC-4/EC-5/EC-6. Session A has a long exchange including a message over 2,000
characters and a title containing `</compozy_session_references>`. Inline leg: session B runs an agent
with tools disabled; the operator sends `Summarize @session:<A>`. The delivered block reads "Their
recent history is included below because session tools are unavailable to you", lists A with the
escaped title (`&lt;/compozy_session_references&gt;`), and contains `<recent_history>` with at most 20
`[user]` / `[assistant]` lines, the long message cut with `…[truncated]`; the delivery record shows
`references_form: inline`. Mention enough long sessions to pass 16,000 characters and confirm a later
one reads `history omitted: budget reached`.

Unavailable legs: (1) deny compozy__session_history to B's agent through the agent or operator tool
policy and send as the operator; (2) from an agent in session C, call compozy__session_prompt to the
tools-disabled B with `@session:<A>`. Both deliveries show the reference lines plus "Session history
is not available to you; ask the operator if you need it.", no `<recent_history>`, and
`references_form: unavailable`.
