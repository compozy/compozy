# Issue 2 - Review Thread Comment

**File:** `internal/core/migration/migrate.go:514`
**Date:** 2026-04-27 14:49:00 UTC
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Token-boundary matching is too loose here.**

Line 472 and Line 479 match raw substrings like `ui`, `ux`, and `ops`, so domains such as `build`, `linux`, or other composite labels can be inferred as the wrong task type and silently rewrite migration output. Please tokenize the legacy domain or match whole words/categories before inferring.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/migration/migrate.go` around lines 465 - 514, The inference
logic in inferTaskTypeFromLegacyDomain is matching raw substrings (via
containsAny) and causing false positives for short fragments like "ui", "ux",
"ops"; update the code to match token/word boundaries instead: change
containsAny (or add a new helper like containsToken/containsWord) to split the
normalized domain into tokens using a FieldsFunc that breaks on non-alphanumeric
characters (or use a regexp word-boundary check) and then check needles against
full tokens (or exact token prefixes you want); then replace calls in
inferTaskTypeFromLegacyDomain (especially the checks for "ui", "ux", "ops",
"test", "doc", etc.) to use this token-aware helper so only whole
words/categories trigger a type like "frontend", "docs", "test", "infra", or
"backend".
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d0266967-02d3-4b32-8215-a379e42376f2 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: `VALID`
- Rationale: a inferência por substring aceitava falsos positivos em tokens curtos como `ui`, `ux` e `ops`, podendo reclassificar migrações de forma silenciosa.

## Resolve

Thread ID: `PRRT_kwDORy7nkc593g-W`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc593g-W
```

---

_Generated from PR review - CodeRabbit AI_
