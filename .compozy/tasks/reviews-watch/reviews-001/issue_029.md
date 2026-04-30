---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: web/src/systems/reviews/components/review-detail-view.tsx
line: 286
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208391746,nitpick_hash:c7d51b7d4c73
review_hash: c7d51b7d4c73
source_review_id: "4208391746"
source_review_submitted_at: "2026-04-30T20:37:25Z"
---

# Issue 029: Consider whether removing all underscores is intentional.
## Review Comment

The `displayReviewTitle` function removes all underscores with `.replace(/_/g, "")`, which might unintentionally mangle technical identifiers in review titles (e.g., `"Fix my_variable_name"` becomes `"Fix myvariablename"`).

If the intent is to convert snake_case to spaces, consider replacing underscores with spaces instead:

## Triage

- Decision: `valid`
- Notes: `displayReviewTitle()` currently strips every underscore, which does clean up markdown emphasis markers but also collapses real identifiers like `my_variable_name`. I will make the sanitization remove markdown-style emphasis delimiters while preserving meaningful interior underscores, and I will add a regression test around the rendered title.
- Resolution: Reworked the title sanitizer to strip markdown emphasis delimiters without collapsing identifier underscores, added a rendered-title regression test, and verified it in the full pipeline.
