---
status: resolved
file: packages/ui/tests/package-exports.test.ts
line: 40
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:f3efb9ebcf97
review_hash: f3efb9ebcf97
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 028: Consider tightening the leak-detection regex.
## Review Comment

Current substring matching can produce false positives for legitimate names. Word boundaries (or a curated denylist) will make this test less brittle.

## Triage

- Decision: `valid`
- Notes:
  - The review is valid. The current leak-detection check uses raw substring matching, which can flag legitimate exports when a banned token only appears inside a larger word.
  - Root cause: export names are checked with a broad regex instead of matching route-specific terms as full naming segments.
  - Fix approach: tokenize export names on PascalCase and separator boundaries, then match only full route-specific tokens so legitimate names are not false positives.

## Resolution

- Reworked `packages/ui/tests/package-exports.test.ts` to tokenize export names on case and separator boundaries before checking for route-specific leaks.
- Added explicit regression coverage showing `RuntimeBadge` does not trip the leak detector while `RunDetailView` still does.
- Verification:
- `bunx vitest run --config vitest.config.ts tests/primitives.test.tsx tests/package-exports.test.ts tests/tokens.test.ts tests/storybook-stories.test.tsx` (from `packages/ui`)
- `make verify`
