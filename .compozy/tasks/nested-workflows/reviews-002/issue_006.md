---
provider: manual
pr:
round: 2
round_created_at: 2026-07-22T15:39:03Z
status: resolved
file: skills/cy-create-tasks/references/task-template.md
line: 65
severity: medium
author: claude-code
provider_ref:
---

# Issue 006: Persistence criteria lack measurable assertions

## Review Comment

The template asks for generic “measurable outcomes” but does not require persistence constraints to name the observable property or measurement mechanism. Phrases such as “one authorized result,” “preserve semantic order,” or “efficient query” can be copied into a generated task and satisfied by tests that inspect constants rather than runtime database behavior.

Represent each persistence constraint with a distinct property, target, and measurement mechanism. Supported assertions should include exact or bounded SQL statement counts, reads within one snapshot transaction, shared row/total predicates, generated query-plan checks, and statement-observer evidence. Reject any persistence criterion without an observable assertion. Keep consistency/correctness constraints separate from performance thresholds, and require tests to observe runtime behavior rather than implementation constants.

## Triage

- Decision: `VALID`
- Notes: `task-template.md` requires only generic measurable outcomes, while
  `skills/cy-create-tasks/SKILL.md` repeats that generic requirement without
  defining an assertion contract for persistence work. A generated task can
  therefore restate a desired quality without naming the database behavior,
  target, or runtime observation that proves it. Strengthen the scoped template
  so every persistence constraint has a property, target, and measurement
  mechanism; separate correctness/consistency assertions from performance
  thresholds; list supported runtime evidence; and reject criteria that inspect
  implementation constants instead of executed database behavior.
- Regression coverage: `TestTaskTemplateRequiresObservablePersistenceCriteria`
  reads the shipped template and checks the required criterion shape, category
  separation, runtime evidence mechanisms, and rejection rules.
