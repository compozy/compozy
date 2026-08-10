---
name: [skill-name]
description: [Leading word first, then one trigger per branch. Third person, max 1,024 characters. Use when [positive triggers]. Don't use for [negative triggers].]
---

# [Skill Title]

[One or two lines: the behaviour this skill makes predictable.]

<!-- A skill is steps, reference, or both. Keep the section(s) that fit; delete the other(s). -->

## Steps — [for sequenced work]

**Step 1: [Action Phase]**
1. [Third-person imperative instruction, e.g., "Extract the query parameters…"]
2. [Pointer worded for when, e.g., "When the output must match a schema, read `assets/template.json` in full."]

*Done when:* [checkable criterion — done vs. not-done is decidable; exhaustive where it matters, e.g., "every modified model accounted for"].

**Step 2: [Action Phase]**
1. [Conditional logic, e.g., "If source maps are required, run `scripts/build.sh`; otherwise skip to Step 3."]
2. Execute `python [skill-dir]/scripts/[script-name].py` to [deterministic action] ([read-only | bootstrap | mutating]).

*Done when:* [checkable criterion].

## Reference — [for rules consulted on demand; a flat peer-set is a valid shape for a whole skill]

- [Rule or fact, with its caveats co-located under the same heading.]
- [Pointer worded for when: "When [trigger], read `references/[file].md` in full."]

## Error Handling

<!-- Only if the skill runs scripts or has known failure states — otherwise delete this section. -->
* If `scripts/[script-name].py` fails due to [specific edge case], [recovery step].
