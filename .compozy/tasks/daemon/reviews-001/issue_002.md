---
status: resolved
file: internal/api/client/client.go
line: 167
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mmv,comment:PRRC_kwDORy7nkc65HKX5
---

# Issue 002: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Escape `slug` before composing the request path.**

Every other path parameter in this client is escaped, but this one isn't. A slug containing `/`, `?`, `#`, or spaces will hit the wrong route or fail request validation.

<details>
<summary>Suggested fix</summary>

```diff
-	path := "/api/tasks/" + slug + "/runs"
+	path := "/api/tasks/" + url.PathEscape(slug) + "/runs"
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/client/client.go` around lines 150 - 167, The slug variable is
not URL-escaped before being embedded into the request path, so pass the slug
through url.PathEscape and use that escaped value when building path (replace
the concatenation that creates path := "/api/tasks/"+slug+"/runs" with a version
that uses url.PathEscape(slug)); ensure the net/url import is present and then
call c.doJSON as before with the escaped path so requests with slashes, spaces,
or special chars route correctly.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4748c55f-38a6-4940-81c9-cabca13fdd92 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `StartTaskRun` concatenates the workflow slug directly into the request path while other client methods escape path parameters first.
- Fix plan: apply `url.PathEscape` before building `/api/tasks/<slug>/runs` and add a regression test covering slugs with spaces and `/`.
- Resolution: Implemented and verified with `make verify`.
