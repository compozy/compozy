---
name: cy-final-verify
description: "Assess a spec task or delivery claim against its contract and available verification evidence."
---

# Verify the Requested Result

Identify the requested outcome, verification boundary, and remaining obligations.
Use this inside the caller's existing validation step; it does not start another
implementation, review, test, or QA cycle or authorize publishing.

Compare the result with the accepted contract and user goal, using already-grounded
canonical examples and acceptance criteria. Pick the cheapest owning suite, probe,
or artifact check that can expose a missing or failed behavior. Inspect the actual
result and exit status. Reuse evidence while its source, dependencies, build/config,
and environment remain applicable; a new message, handoff, or commit alone does
not invalidate it. An unchanged HEAD does not prove a dirty tree was checked.

Repository delivery policy owns commit/push gates and required CI at the current
head. Reuse the project's valid cached checks; do not invent an additional full
local run. A focused result supports its focused claim. A loop checkpoint names
pending integration/visual checks and their owner without claiming they ran; the
workflow resolves them before final delivery.

Read only the applicable reference when its details are needed:

- [Conditional contracts](references/conditional-contracts.md): the matching bug,
  spec, user-visible QA, named visual reference, or created-lab section.
- [PR delivery](references/pr-delivery.md): when commit/push/PR delivery is requested
  and the caller does not already own that procedure.

Finish when the requested result and applicable checks are satisfied. Repair
failures at their source and repeat only invalidated checks; preserve valid tests
and warning/error policy. Continue authorized repairs without a review pause after
the first implementation. Report a concrete blocker when a missing external input
prevents completion.

Summarize the outcome, evidence, and material limits in the caller's existing
report. Link larger artifacts when needed; do not create another checklist/report
or broaden validation solely because a companion skill is installed.
