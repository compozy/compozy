# Issue 1 - Review Thread Comment

**File:** `internal/core/migration/migrate.go:475`
**Date:** 2026-04-27 15:04:12 UTC
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Tighten the docs matcher to avoid `Docker` → `docs`.**

Line 474 treats any `doc*` token as documentation, so domains like `Docker` or `docstore` will be migrated to `type: docs` before the `infra`/`backend` branches are even considered. That silently rewrites some legacy feature tasks to the wrong v2 type.

<details>
<summary>Suggested fix</summary>

```diff
-	case registry.IsAllowed("docs") && hasAnyTokenPrefix(tokens, "doc"):
+	case registry.IsAllowed("docs") &&
+		hasAnyToken(tokens, "doc", "docs", "documentation"):
 		return "docs"
```

</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/migration/migrate.go` around lines 474 - 475, The current case
using registry.IsAllowed("docs") && hasAnyTokenPrefix(tokens, "doc") incorrectly
matches tokens like "Docker" and "docstore"; change the matcher to only match
exact documentation tokens (e.g., "doc", "docs", "documentation") instead of any
prefix. Update the condition in the migrate switch branch (the case that returns
"docs") to call an exact-token checker (or extend/replace hasAnyTokenPrefix with
a check like hasAnyTokenExact(tokens, "doc", "docs", "documentation") or a regex
that anchors the whole token) so only true documentation tokens trigger return
"docs".
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4af99e3f-f4cd-439d-9de3-a76708043e62 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: `VALID`
- Rationale: o matcher de documentação por prefixo aceitava tokens como `Docker` e `docstore`, o que podia migrar tarefas para `docs` de forma incorreta.

## Triage

- Disposition: `VALID`
- Rationale: o matcher de `docs` por prefixo aceitava tokens como `Docker`, o que pode reclassificar incorretamente tarefas legadas antes dos ramos `infra`/`backend`.

## Resolve

Thread ID: `PRRT_kwDORy7nkc5930EP`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc5930EP
```

---

_Generated from PR review - CodeRabbit AI_
