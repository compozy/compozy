---
id: ET-spec-authoring-slices-review
area: ET
title: Author a spec with slices and optional peer review
persona: Bruno
journey: J-deliver-through-agents
expected: cy-create-spec writes _slices.md, asks the peer review questions once after approval, stops at the first round without blockers, and declining review leaves no review artifacts.
entry_points: skill cy-create-spec; .compozy/tasks/<slug>/_slices.md; .compozy/tasks/<slug>/qa/peer-review-findings-round<N>-<reviewer>.md
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-spec-conductor-delivery
---

Spec QA-003, covering US-004.

Author one small spec. Confirm `_slices.md` lists slice, layer, dependencies, scope and spec references.
After approval, the agent asks once for review, reviewers with a runtime each, and maximum rounds, with
1 as the default.

Run one clean round and confirm no later round starts. Run a second spec with a blocker and two rounds
allowed: the later round covers changed blockers only, and a blocker that needs an operator decision is
escalated. Interrupt a reviewer and confirm its findings file keeps what it wrote. Decline review once
and confirm no review artifact appears.
