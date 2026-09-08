---
name: deslop
description: Review a changed diff for accidental complexity or generated-code noise. Use when requested or when the diff shows those symptoms; an equivalent review of unchanged inputs need not repeat.
---

# Review the Changed Diff

Use the task's actual diff, including relevant working changes. Preserve unrelated edits and project compatibility requirements.

Remove accidental noise introduced by this task: redundant comments, unjustified casts/suppressions, defensive branches on trusted data, duplicated helpers, and unnecessary nesting. Keep boundary validation, error handling, and required migration/deprecation adapters.

Review once before final verification. A review already performed on the same diff satisfies this step; revisit only subsequent relevant edits. Avoid cleanup outside the requested scope. Report substantive changes briefly.
