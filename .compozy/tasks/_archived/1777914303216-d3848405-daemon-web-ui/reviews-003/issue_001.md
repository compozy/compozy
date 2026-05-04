---
status: resolved
file: Makefile
line: 161
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149317620,nitpick_hash:1afb06afda29
review_hash: 1afb06afda29
source_review_id: "4149317620"
source_review_submitted_at: "2026-04-21T16:29:30Z"
---

# Issue 001: Reuse $(BINARY_DIR) and $(BINARY_NAME) in dev.
## Review Comment

Hard-coding `./bin/compozy` duplicates the build config and can drift from `go-build`.

## Triage

- Decision: `valid`
- Notes:
- `Makefile:161-162` still hard-codes `./bin/compozy` even though the build output is already defined by `$(BINARY_DIR)` and `$(BINARY_NAME)` at the top of the file.
- Root cause: the `dev` target duplicated the binary path instead of reusing the canonical build variables, so it can drift from `go-build`.
- Fix plan: update the recipe to invoke `./$(BINARY_DIR)/$(BINARY_NAME)` so the development launcher stays coupled to the configured build output.
- Implemented: `dev` now launches `./$(BINARY_DIR)/$(BINARY_NAME) daemon start --foreground --web-dev-proxy http://127.0.0.1:3000`.
- Minimal extra test scope: updated `test/frontend-workspace-config.test.ts` because the repository's config contract explicitly asserts the Makefile dev entrypoint string.
- Verification:
- `make -n dev`
- `bun run test:config`
- `make verify`
