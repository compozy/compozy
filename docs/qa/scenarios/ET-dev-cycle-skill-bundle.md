---
id: ET-dev-cycle-skill-bundle
area: ET
title: Use the bundled dev-cycle skills in a managed session
persona: Ada
journey: J-offer-runnable-capabilities
expected: A managed session lists and prompts with exactly the nine dev-cycle workflow skills, each view returns its bundled body, and a workspace-local override wins only in its owning workspace while another workspace keeps the global bundled declaration.
entry_points: compozy extension list; compozy skill list|view; compozy__skill_list|view; managed session prompt
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-003; ET-004; ET-skill-activation-gates
---

QA impact 2026-07-27: the bundled dev-cycle extension now publishes its nine workflow skills
globally to Compozy-managed sessions while preserving workspace-local lookup, cache, and prompt
isolation. Planning flag only; the next QA cycle owns execution.
