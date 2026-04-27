# Issue 3 - Review Thread Comment

**File:** `internal/core/migration/migrate.go:528`
**Date:** 2026-04-27 14:49:00 UTC
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Restrict XML tag extraction to the legacy metadata block.**

This helper searches the entire markdown body. If a legacy task is missing `<domain>` in `<task_context>` but later documents `<domain>...</domain>` as an example, migration will treat that body text as metadata and infer a type instead of leaving it unmapped. Search inside `<task_context>` first.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/migration/migrate.go` around lines 516 - 528, The
extractLegacyXMLTag function currently scans the entire markdown body and should
be restricted to the legacy metadata block: modify extractLegacyXMLTag to first
locate the "<task_context>...</task_context>" block (if present) and set that
substring as the search target, then perform the existing openTag/index logic
within that target; if no <task_context> block exists, fall back to the original
full-content search so behavior is unchanged. Ensure references to the function
name extractLegacyXMLTag and the "<task_context>" marker are used to locate
where to change the logic.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d0266967-02d3-4b32-8215-a379e42376f2 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: `VALID`
- Rationale: o helper de extração XML podia ler tags no corpo markdown fora de `<task_context>`, contaminando os metadados legados usados na migração.

## Resolve

Thread ID: `PRRT_kwDORy7nkc593g-b`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc593g-b
```

---

_Generated from PR review - CodeRabbit AI_
