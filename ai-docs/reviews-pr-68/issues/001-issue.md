# Issue 1 - Review Thread Comment

**File:** `internal/core/run/journal/journal.go:115`
**Date:** 2026-04-06 10:19:04 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** `Open()` realmente reaproveitava `events.jsonl` sem recuperar o último `Seq` nem truncar tail parcial. O fix aplicado agora recupera o último registro íntegro, corta a linha final truncada antes de reabrir o writer e garante monotonicidade de sequência em reaberturas/resume.

## Body

_⚠️ Potential issue_ | _🔴 Critical_

**Reopening an existing journal breaks replay invariants.**

`Open` appends to any existing `events.jsonl`, but `seq` always starts at zero again and there is no recovery for a truncated final line. Resumed runs can emit duplicate `Seq` values, and a crash-cut tail will be concatenated with the next JSON object.



Also applies to: 236-244, 306-323

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/journal/journal.go` around lines 96 - 115, Open currently
appends to events.jsonl without recovering state; fix Open (and analogous code
at the other mentions) to scan the existing file before starting writeLoop: read
the file line-by-line, drop any partial/truncated final line (truncate the file
to the last full newline if needed), parse the last valid JSON record to extract
its Seq and initialize the Journal's sequence counter (e.g., set j.seq or
j.nextSeq = lastSeq+1) so resumed runs continue with monotonically increasing
Seq; ensure writeLoop(file) is then started with the truncated file handle so
subsequent writes append correctly and avoid duplicate Seq values.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4154d49c-3d7e-4c97-8137-5703c38da1a3 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55CGQp`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55CGQp
```

---
*Generated from PR review - CodeRabbit AI*
