# Issue 12 - Review Thread Comment

**File:** `go.mod:126`
**Date:** 2025-10-27 13:58:52 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Document accepted AWS S3 Crypto SDK vulns outside go.mod.**

Acceptance note is fine, but prefer moving rationale to SECURITY.md (or a tracking issue) rather than comments in go.mod. Keep the module line clean.

<details>
<summary>🧰 Tools</summary>

<details>
<summary>🪛 OSV Scanner (2.2.3)</summary>

[LOW] 126-126: github.com/aws/aws-sdk-go 1.55.6: In-band key negotiation issue in AWS S3 Crypto SDK for golang in github.com/aws/aws-sdk-go

(GO-2022-0635)

---

[LOW] 126-126: github.com/aws/aws-sdk-go 1.55.6: CBC padding oracle issue in AWS S3 Crypto SDK for golang in github.com/aws/aws-sdk-go

(GO-2022-0646)

</details>

</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In go.mod around lines 124 to 126, the acceptance rationale for AWS S3 Crypto
SDK vulnerabilities is embedded as comments in the module file; move that
rationale into SECURITY.md (or a tracked GitHub issue) and reference it from
go.mod if needed. Remove the comment block from go.mod so the file only contains
the module dependency line, create or update SECURITY.md with the full
explanation (including grep verification, vuln IDs GO-2022-0635/GO-2022-0646,
and justification), and optionally add a one-line comment in go.mod pointing to
SECURITY.md or the issue number for traceability.
```

</details>

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5fez7-`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5fez7-
```

---
*Generated from PR review - CodeRabbit AI*
