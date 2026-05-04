---
status: resolved
file: packages/ui/src/tokens.css
line: 14
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4167198630,nitpick_hash:b1ba131f7bab
review_hash: b1ba131f7bab
source_review_id: "4167198630"
source_review_submitted_at: "2026-04-24T01:14:40Z"
---

# Issue 008: Prefer WOFF2 for shipped mono fonts
## Review Comment

Using `.ttf` for web delivery is heavier; WOFF2 variants would reduce font payload and improve first render performance.

Also applies to: 22-23

## Triage

- Decision: `invalid`
- Notes:
  - The repository currently ships `Disket Mono` only as `.ttf` assets under `packages/ui/src/assets/fonts`; there are no corresponding `.woff2` files to reference.
  - Updating `tokens.css` alone would point the app at missing files and break font loading instead of improving it.
  - Generating and validating new font assets is outside this scoped review batch, so this suggestion is deferred.
