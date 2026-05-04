---
status: resolved
file: internal/api/httpapi/security_headers.go
line: 18
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc59RNfX,comment:PRRC_kwDORy7nkc662oP_
---

# Issue 001: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Tighten CSP: avoid allowing inline scripts by default.**

`script-src 'self' 'unsafe-inline'` significantly reduces CSP’s XSS protection value. Prefer `script-src 'self'` and use nonces/hashes only where truly required.


<details>
<summary>🔐 Proposed change</summary>

```diff
-		"script-src 'self' 'unsafe-inline'; " +
+		"script-src 'self'; " +
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
"script-src 'self'; " +
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/security_headers.go` at line 18, The CSP currently
allows inline scripts via "script-src 'self' 'unsafe-inline'" which weakens XSS
protection; locate the constant or function that builds the
Content-Security-Policy header (e.g., the variable/content string named
contentSecurityPolicy or the function that returns security headers in
security_headers.go) and remove 'unsafe-inline' from the script-src directive so
it reads only "script-src 'self'". If your app requires specific inline scripts,
replace unsafe-inline with per-response nonces or explicit script hashes and
ensure the code that injects headers (e.g., BuildSecurityHeaders or
GetSecurityHeaders) generates and inserts those nonces/hashes into both the CSP
and the script tags where used.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:cc1bd397-1de5-475f-a301-c04207716559 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `web/dist/index.html` only ships an external module script and does not contain inline `<script>` blocks, so the daemon UI does not need `'unsafe-inline'` in `script-src`.
  - Keeping `'unsafe-inline'` in `script-src` weakens the CSP without a compatibility requirement and reduces XSS protection for the daemon UI.
  - Fix: remove `'unsafe-inline'` from the `script-src` directive and add a middleware regression test that locks the CSP contract.
  - Full-repository verification also surfaced a pre-existing `goconst` lint failure in `internal/daemon/query_service.go`; I applied the minimal constant reuse needed there so the required `make verify` gate could pass.
