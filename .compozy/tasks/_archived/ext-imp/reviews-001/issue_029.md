---
status: resolved
file: sdk/extension-sdk-ts/test/extension.test.ts
line: 312
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0ip,comment:PRRC_kwDORy7nkc624f79
---

# Issue 029: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Align the capability-denied error payload with the Go SDK before locking this in.**

This test asserts `fetch_reviews` capability failures return `{ target, missing, granted }`, while `sdk/extension/internal_test.go` now asserts `{ method, required }` for the same RPC failure. Merging both codifies two different wire-level error schemas for the same extension protocol behavior, which makes cross-SDK host/tooling handling brittle.

Based on learnings "Enforce consistency of patterns and conventions across the system".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@sdk/extension-sdk-ts/test/extension.test.ts` around lines 284 - 312, Test
asserts the capability-denied RPC payload using { target, missing, granted } but
the Go SDK uses { method, required }, so update this test to expect the Go-style
schema: when invoking "fetch_reviews" via invoke("fetch_reviews", ...) assert
the rejection matches an RPCError whose data contains method: "fetch_reviews"
and required: [CAPABILITIES.providersRegister] (and adjust granted/missing
expectations accordingly), updating the shape referenced in the
expect(...).rejects.toMatchObject<RPCError> to use method/required instead of
target/missing/granted so the wire schema matches the Go SDK.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:cfb4e507-6796-4f02-9ecd-afb099e21990 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. The TypeScript SDK still emits capability-denied payloads as `{ target, missing, granted }`, while the Go SDK already uses `{ method, required, granted }`.
  - Root cause: `newCapabilityDeniedError()` in the TypeScript SDK has drifted from the shared protocol shape used by the Go SDK.
  - Intended fix: update the TypeScript production error payload to the Go-aligned schema and adjust the scoped tests to assert the shared wire contract.
  - Resolution: the TypeScript SDK now emits Go-aligned capability-denied payloads; because review-provider registration now auto-declares `providers.register`, the scoped regression now asserts the shared schema on the initialize failure path.
