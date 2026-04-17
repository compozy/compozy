---
status: resolved
file: sdk/extension-sdk-ts/src/extension.ts
line: 185
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0in,comment:PRRC_kwDORy7nkc624f78
---

# Issue 027: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Auto-declare `providers.register` when a review provider is registered.**

Right now `registerReviewProvider()` updates the advertised provider set, but it does not update the negotiated capability set. That leaves the initialize contract inconsistent: `registered_review_providers` can be non-empty while `accepted_capabilities` omits `providers.register`, so the host can reject the extension or later `fetch_reviews` / `resolve_issues` calls fail with capability denial.

<details>
<summary>Suggested fix</summary>

```diff
   registerReviewProvider(name: string, handler: ReviewProviderHandler): this {
     const normalized = name.trim();
     if (normalized !== "") {
       this.reviewProviders.set(normalized, handler);
+      this.declaredCapabilities.add(CAPABILITIES.providersRegister);
     }
     return this;
   }
```
</details>


Based on learnings, maintain clear system boundaries and establish clear ownership of each boundary.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
  /** Registers one executable review provider handler by name. */
  registerReviewProvider(name: string, handler: ReviewProviderHandler): this {
    const normalized = name.trim();
    if (normalized !== "") {
      this.reviewProviders.set(normalized, handler);
      this.declaredCapabilities.add(CAPABILITIES.providersRegister);
    }
    return this;
  }
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@sdk/extension-sdk-ts/src/extension.ts` around lines 178 - 185, When
registerReviewProvider (the method that sets this.reviewProviders) is called
with a non-empty name, also add the 'providers.register' capability to the
extension's negotiated/accepted capability set so the initialize contract
remains consistent; update the property that tracks accepted capabilities (e.g.,
this.acceptedCapabilities or this.negotiatedCapabilities) by inserting
'providers.register' whenever registerReviewProvider registers a handler.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:2b0ec709-3f41-4b6f-b5c8-2840068e728b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `registerReviewProvider()` records the provider name but does not declare `providers.register`, so initialize can advertise review providers without negotiating the required capability.
  - Root cause: the review-provider registration path updates the provider registry but not the capability set used to build `accepted_capabilities`.
  - Intended fix: add `CAPABILITIES.providersRegister` automatically when a non-empty review provider is registered.
  - Resolution: registering a review provider now auto-declares `providers.register`, and the bridge integration test matrix was updated to cover the new TS contract.
