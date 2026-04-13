---
status: resolved
file: sdk/extension-sdk-ts/templates/review-provider/test/review-provider.test.ts
line: 23
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093952670,nitpick_hash:7ca496999294
review_hash: 7ca496999294
source_review_id: "4093952670"
source_review_submitted_at: "2026-04-11T16:18:34Z"
---

# Issue 028: Type casting bypasses SDK type safety.
## Review Comment

The double cast `as unknown as { call<T>(...): Promise<T> }` is fragile. If the `TestHarness` API changes, this cast will silently break at runtime rather than at compile time.

Consider exposing a typed `call` method on `TestHarness` in the SDK, or using a properly typed test utility to avoid runtime surprises.

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. The review-provider template test reaches into `TestHarness` with a double cast because the harness’s typed request helper is private.
  - Root cause: SDK tests need a typed way to issue arbitrary RPC requests, but `TestHarness.call()` is private, forcing unsafe casting in the scoped tests.
  - Intended fix: expose the typed harness request helper and update the scoped tests to use it directly. This requires a minimal non-scoped change in `sdk/extension-sdk-ts/src/testing/test_harness.ts`, which is necessary to remove the unsafe cast at the source.
  - Resolution: exposed `TestHarness.call()` publicly and updated the scoped SDK/template tests to use it directly without unsafe casts.
