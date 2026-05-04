---
status: resolved
file: packages/ui/src/components/button.tsx
line: 35
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:e77b88fe3cfc
review_hash: e77b88fe3cfc
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 023: Enforce accessible naming for icon-only buttons.
## Review Comment

At Lines 51-56, icon-only usage is currently possible without an accessible label. Tightening props prevents silent a11y regressions.

## Triage

- Decision: `valid`
- Notes:
  - The review is valid. `ButtonProps` currently permits `icon` while both `children` and accessible-name props remain optional, so an icon-only button can compile without an accessible name.
  - Root cause: the public prop type is too permissive; the render path only marks the icon span `aria-hidden`, so accessibility depends entirely on the caller providing a label.
  - Fix approach: tighten the button prop type so icon-only usage requires `aria-label` or `aria-labelledby`, then add regression coverage for the labeled icon-only case and the type-level restriction.

## Resolution

- Tightened `packages/ui/src/components/button.tsx` so icon-only buttons now require `aria-label` or `aria-labelledby`.
- Added regression coverage in `packages/ui/tests/primitives.test.tsx` for labeled icon-only rendering plus the type-level rejection of unlabeled icon-only props.
- Made one minimal downstream compatibility adjustment in `packages/ui/src/components/stories/button.stories.tsx` so Storybook stories provide button children through `render` rather than `args`, preserving the stricter prop contract without weakening the fix.
- Verification:
- `bun run --cwd packages/ui typecheck`
- `bunx vitest run --config vitest.config.ts tests/primitives.test.tsx tests/package-exports.test.ts tests/tokens.test.ts tests/storybook-stories.test.tsx` (from `packages/ui`)
- `make verify`
