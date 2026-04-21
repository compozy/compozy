---
status: resolved
file: scripts/codegen.mjs
line: 70
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:102a58e3af03
review_hash: 102a58e3af03
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 030: Keep --check mode read-only.
## Review Comment

`--check` currently rewrites `web/src/generated/compozy-openapi.d.ts` before failing. Consider making check mode non-mutating and leaving writes to normal mode only.

## Triage

- Decision: `valid`
- Notes:
  - The review is valid. `scripts/codegen.mjs --check` compares a regenerated temp file to the checked-in target, but when they differ it still writes the new contents back to `web/src/generated/compozy-openapi.d.ts` before exiting non-zero.
  - Root cause: check mode reuses the write-path side effect after drift detection instead of staying read-only.
  - Fix approach: keep check mode read-only, report the mismatch without mutating the tracked file, and add focused regression coverage for the script path. That test will need a new root-level test file because no existing script test covers this behavior.

## Resolution

- Removed the write side effect from `scripts/codegen.mjs --check`; drift is now reported without mutating `web/src/generated/compozy-openapi.d.ts`.
- Added focused regression coverage in `test/codegen-script.test.ts` for both read-only `--check` mode and normal write mode.
- Verification:
- `bunx vitest run --config vitest.config.ts test/codegen-script.test.ts`
- `make verify`
