---
status: resolved
file: internal/core/workspace/config.go
line: 68
severity: medium
author: claude-code
provider_ref:
---

# Issue 002: ExecConfig and validators duplicate DefaultsConfig verbatim

## Review Comment

The new `ExecConfig` struct and its eight validator helpers are direct
copies of `DefaultsConfig` and its counterparts. Every field and every
validation rule is identical between the two sections, with only the error
message prefix (`defaults.*` vs `exec.*`) changed.

Duplicated structs:
- `DefaultsConfig` (config.go:36-48) vs `ExecConfig` (config.go:68-80) —
  identical 11-field shape.

Duplicated validator pairs (config.go:275-457):
- `validateDefaultIDE` / `validateExecIDE`
- `validateDefaultOutputFormat` / `validateExecOutputFormat`
- `validateDefaultReasoningEffort` / `validateExecReasoningEffort`
- `validateDefaultAccessMode` / `validateExecAccessMode`
- `validateDefaultTimeout` / `validateExecTimeout`
- `validateDefaultTailLines` / `validateExecTailLines`
- `validateDefaultMaxRetries` / `validateExecMaxRetries`
- `validateDefaultRetryBackoffMultiplier` / `validateExecRetryBackoffMultiplier`

This is ~160 lines of copy-paste that must be updated in both places every
time a validation rule changes (e.g., adding a new reasoning-effort tier,
tightening a timeout bound). That maintenance drift is the main risk — a
future edit to `defaults.*` validation will silently diverge from `exec.*`.

**Suggested fix:** introduce a shared `RuntimeOverrides` (or similar) struct
that both `DefaultsConfig` and `ExecConfig` embed, plus a single
`validateRuntimeOverrides(fieldPrefix string, cfg RuntimeOverrides) error`
that both sections call. The field-prefix parameter produces the correct
error message without duplicating the validation body.

## Triage

- Decision: `valid`
- Notes:
  Root cause confirmed in `internal/core/workspace/config.go`: `ExecConfig` duplicates the entire runtime override shape from `DefaultsConfig`, and the exec/default validators repeat the same rules with only the field prefix changed.
  The maintenance risk is real because any new runtime override field or validation rule now has to be edited in two places, with no shared compiler-enforced contract between them.
  Fix approach: extract the shared runtime override fields into one reusable struct and centralize validation behind a prefix-aware helper, then cover both defaults and exec sections in `internal/core/workspace/config_test.go`.
  Verified with focused package tests and a clean `make verify`; `internal/core/workspace/config_test.go` now exercises shared validation through both `defaults.*` and `exec.*` error paths.
