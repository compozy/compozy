---
name: lesson-learned
description: Extract evidence-backed engineering lessons from recent changes or incidents. Use for requested reflection, lesson updates, or retiring an obsolete workflow rule.
---

# Lesson Learned

Extract or revise an engineering lesson from concrete incident evidence, code changes, and user decisions. Keep the causal explanation and the narrow rule it supports.

1. Resolve the requested commit, diff, incident, or lesson scope from context. Reuse gathered evidence; inspect additional files only to answer a specific causal question.
2. Separate incident facts, current policy, and historical practice. Check whether a newer directive supersedes the original assumption before promoting it to a rule.
3. State what happened, the confirmed cause, the consequence, and the smallest reusable decision that prevents recurrence. Cite local paths/commits. Record uncertainty rather than forcing a lesson from weak evidence.
4. Keep an existing lesson's ID and source trail when its policy changes. Mark obsolete guidance as historical, update the current application and index, and link the authoritative replacement.
5. Check referenced files and policy consistency. Editorial lesson changes need no tests that freeze prose and no new runtime QA cycle.

Read the matching entry in `references/se-principles.md` only when a named principle helps explain the evidence; `references/anti-patterns.md` is optional diagnostic background. A lesson does not automatically require a new skill, test file, approval, review round, or whole-project gate.

Present the finding in a short explanation or the repository's established lesson format. Include current scope and supersession conditions for procedural guidance. Avoid generic advice, mandatory templates for trivial changes, and claims that an old incident proves a model-wide limitation.
