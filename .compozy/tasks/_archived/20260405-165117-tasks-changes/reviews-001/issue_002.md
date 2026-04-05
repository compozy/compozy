---
status: resolved
file: internal/core/model/model.go
line: 184
severity: medium
author: claude-code
provider_ref:
---

# Issue 002: Empty dependencies are dropped from rewritten task files

## Review Comment

`TaskFileMeta.Dependencies` is tagged with `yaml:"dependencies,omitempty"`, so every rewrite through `frontmatter.Format(model.TaskFileMeta{...})` drops the field when the list is empty. That means the new migrate paths rewrite valid tasks without `dependencies: []`, even though the documented v2 schema and the `cy-create-tasks` template require the field to be present when there are no dependencies. The resulting files still parse today, but they drift away from the declared task contract and from the migration requirement to preserve dependencies as-is.

Remove `omitempty` from `Dependencies` and add a migration/output assertion so rewritten task files preserve `dependencies: []` instead of silently omitting the field.

## Triage

- Decision: `VALID`
- Root cause: `internal/core/model/model.go` tags `TaskFileMeta.Dependencies` with `yaml:"dependencies,omitempty"`, so every rewrite through `frontmatter.Format(model.TaskFileMeta{...})` drops the field when the dependency slice is empty.
- Impact: migrated v2 task files no longer preserve the documented `dependencies: []` contract, which creates schema drift even when the task had an explicit empty dependency list before the rewrite.
- Fix approach: remove `omitempty` from `Dependencies` and add regression coverage that asserts rewritten task files still emit `dependencies: []` for empty dependency lists.
- Resolution: `TaskFileMeta.Dependencies` now always serializes, and migration regression tests assert rewritten task files preserve `dependencies: []` across both v1 and legacy migration paths.
- Verification: `go test ./internal/core/tasks ./internal/core ./internal/core/model` passed, then `make verify` passed cleanly.
