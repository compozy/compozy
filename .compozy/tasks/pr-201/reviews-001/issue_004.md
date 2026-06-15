---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/cli/task_runtime_form.go
line: 75
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhpp,comment:PRRC_kwDORy7nkc7LlP2i
---

# Issue 004: _⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_

**Single-workflow mode can drop workflow-scoped runtime overrides.**

Line 53 sets `scopeWorkflow` only when `len(slugs) > 1`, which makes selector keys unscoped for single-workflow runs. That prevents lookup/preseeding of existing workflow-qualified rules (lines 133-143, 170-174), and the rebuilt rule set can lose them on apply.

<details>
<summary>💡 Suggested direction</summary>

```diff
- scopeWorkflow := len(slugs) > 1
+ displayWorkflow := len(slugs) > 1

...
- if err := form.populate(workflow, entries, typeRuleByValue, taskRuleByID, scopeWorkflow); err != nil {
+ if err := form.populate(workflow, entries, typeRuleByValue, taskRuleByID, displayWorkflow); err != nil {
```

Then inside option creation:
- keep `optionWorkflow` scoped to the real workflow for keys/rule matching,
- use a separate display workflow (empty when single-select) only for labels.
</details>






Also applies to: 133-143, 170-174

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/cli/task_runtime_form.go` around lines 53 - 75, The scopeWorkflow
variable at line 53 is set to false for single-workflow runs (when len(slugs) <=
1), which prevents proper lookup and preseeding of workflow-qualified runtime
rules at lines 133-143 and 170-174, causing existing workflow-scoped overrides
to be lost. Change the logic to always use the actual workflow name for rule
matching and key generation, regardless of the number of workflows. Introduce a
separate display-only workflow variable (which can be empty for single-workflow
mode) while keeping the workflow parameter used in the populate call and rule
matching logic consistently scoped to the real workflow name. Update the
form.populate call and all rule lookup/matching logic at the cited lines to use
the real workflow name for keys and matching, with display labels using only the
display variable.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:0f3816f6d848434b318d4af7 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `newTaskRunRuntimeFormForSlugs` uses `scopeWorkflow := len(slugs) > 1` for both display labels and selector keys, so a single-workflow run cannot preselect workflow-qualified runtime rules and can rebuild them as unscoped rules.
- Fix approach: separate rule identity from display labeling: always use the real workflow for keys/lookups/rule output, and use a display-only workflow value for labels when more than one workflow is shown.

## Resolution

- Resolved by preserving workflow scope separately from display labels.
- Verification: `rtk make verify` exited 0 after the code changes.
