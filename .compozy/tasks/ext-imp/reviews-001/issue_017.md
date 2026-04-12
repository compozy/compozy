---
status: resolved
file: internal/core/provider/coderabbit/nitpicks_test.go
line: 281
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0iZ,comment:PRRC_kwDORy7nkc624f7t
---

# Issue 017: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Strengthen the outcome assertions here.**

`len(items) == 2` plus non-empty hashes will still pass if the parser returns the wrong two review-body comments. Since this change is about category expansion, please assert the expected titles/severities (or equivalent stable fields) for both returned items.


As per coding guidelines, "Ensure tests verify behavior outcomes, not just function calls."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/provider/coderabbit/nitpicks_test.go` around lines 276 - 281,
The test currently only asserts len(items) == 2 and non-empty ReviewHash, which
can pass with wrong items; update the assertions in nitpicks_test.go to verify
stable, meaningful fields for each returned item (e.g., Title, Severity,
Category or equivalent) in addition to ReviewHash so the test validates the
specific expected review-body comments; locate where items is built and add
assertions that items[0].Title (and/or items[0].Severity) equals the expected
value(s) and the same for items[1] to ensure the parser returns the correct two
items.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:2b0ec709-3f41-4b6f-b5c8-2840068e728b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `TestFetchReviewsIncludesReviewBodyCommentsWhenRequested` only checks the item count and non-empty hashes, so it can pass even if the parser returns the wrong two review-body comments.
  - Root cause: the regression test does not assert stable semantic fields for the returned items.
  - Intended fix: assert the expected titles/severities for both parsed comments in addition to the review hashes.
  - Resolution: strengthened the fetched nitpick assertions to validate the expected titles and severities for both returned items.
