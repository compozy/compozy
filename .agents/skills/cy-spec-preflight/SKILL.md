---
name: cy-spec-preflight
description: "Check a Compozy spec or task artifact during authoring or when its contracts change."
---

# Spec Preflight

Check the current artifact within its authoring/change step. Reuse accepted
research, decisions, and prior checks; execution of an unchanged task does not
require another preflight or repository survey.

Choose only the relevant branch:

- **Spec:** [Spec contracts](references/spec-contracts.md) for outcome, interfaces,
  compatibility, and applicable companion contracts. Use the playbook only for an
  unresolved authoring convention and `references/phase-lessons.md` when a matching
  design question needs historical evidence.
- **Tasks/task body:** [Task checks](references/tasks-checks.md) for graph consistency,
  outcome coverage, canonical references, test/evidence ownership, and QA scope.

Repair substantive contract or dependency gaps at their owner, then check the
changed facts. User decisions and repository compatibility policy outrank stale
artifacts; an ADR cannot silently narrow the accepted outcome. Missing template
phrases, empty optional sections, or an unused companion document are not blockers.
Ask only when a missing product decision prevents correct authoring. Peer review
of an approved spec remains opt-in.
