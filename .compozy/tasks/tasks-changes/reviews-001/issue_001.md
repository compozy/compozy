---
status: resolved
file: internal/core/tasks/type_remap.go
line: 33
severity: high
author: claude-code
provider_ref:
---

# Issue 001: Legacy type remap ignores the active task registry

## Review Comment

`RemapLegacyTaskType()` returns the hard-coded built-in mapping immediately, without checking whether the mapped slug is allowed by the workspace's resolved `TypeRegistry`. In a workspace that overrides `[tasks].types`, values such as `"Documentation"` migrate to `docs` even when `docs` is not allowed. `migrate` then treats the file as clean because the follow-up path only flags empty types (`internal/core/migrate.go`), so the command exits successfully and omits the fix prompt even though `validate-tasks` fails immediately afterward.

Use the registry as the final authority here: after applying an explicit remap, return the mapped slug only when `registry.IsAllowed(mapped)` is true; otherwise fall back to `""`. The migration-side unmapped check should use the same rule so the summary and `UnmappedTypeFiles` stay accurate for custom task taxonomies.

## Triage

- Decision: `VALID`
- Root cause: `internal/core/tasks/type_remap.go` returns the hard-coded remap result immediately, so built-in slugs such as `docs` bypass the active `TypeRegistry` even when the workspace overrides `[tasks].types`.
- Impact: `internal/core/migrate.go` rewrites the task as if the type were valid, does not add the file to `UnmappedTypeFiles`, and omits the follow-up fix prompt even though `validate-tasks` will reject the migrated file.
- Fix approach: make the registry the final authority for both explicit remaps and case-insensitive passthroughs, then add migration coverage for a workspace with custom task types so disallowed legacy mappings become `type: ""` and stay listed as unmapped.
- Resolution: `RemapLegacyTaskType()` now returns an explicit remap only when the active registry allows the mapped slug, and new regression coverage proves `migrate` keeps disallowed remaps unmapped under custom task taxonomies.
- Verification: `go test ./internal/core/tasks ./internal/core ./internal/core/model` passed, then `make verify` passed cleanly.
