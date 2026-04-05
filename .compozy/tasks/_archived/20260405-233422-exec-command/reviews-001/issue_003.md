---
status: resolved
file: internal/core/model/model.go
line: 74
severity: low
author: claude-code
provider_ref:
---

# Issue 003: RuntimeConfig.SystemPrompt is unreachable from public API

## Review Comment

`model.RuntimeConfig.SystemPrompt` (model.go:74) is only read by the exec
job path:

```go
// internal/core/plan/prepare.go:234
SystemPrompt: strings.TrimSpace(cfg.SystemPrompt),
```

But `core.Config.runtime()` in `internal/core/api.go:273-306` never copies
any value into `runtimeCfg.SystemPrompt`, and `core.Config` itself does not
expose a `SystemPrompt` field. So `cfg.SystemPrompt` is always empty for
exec mode and the `strings.TrimSpace` call is dead code that implies a
configurability the public API does not offer.

For PRD-tasks mode, `buildBatchJob` sets `SystemPrompt` directly from
`prompt.BuildSystemPromptAddendum(params)` without reading the runtime
field, so the field is unused in that path too.

**Suggested fix:** decide what you want `SystemPrompt` to mean and either
(a) remove the unused read in `buildExecJob` and delete the field from
`RuntimeConfig` so the surface area matches reality, or (b) plumb a
`SystemPrompt` field through `core.Config` and `runtime()` so exec mode
callers can actually set it. Leaving the half-wired field in place just
invites future confusion.

## Triage

- Decision: `valid`
- Notes:
  Root cause confirmed by tracing the public API and planner together. `internal/core/api.go` never populates a `SystemPrompt` value on `model.RuntimeConfig`, so the exec-only read in `buildExecJob` is unreachable from the exported `core.Config` surface.
  The field is still needed on `model.Job` because the executor prepends job-scoped system prompts that are built for PRD-task batches, but the runtime-level field is dead configuration state with no supported caller path.
  Fix approach: remove `RuntimeConfig.SystemPrompt` and the dead exec-mode read, then update tests so the supported system-prompt behavior remains covered through `model.Job.SystemPrompt` instead of an unreachable runtime setting.
  Minimal non-scope production edit: `internal/core/plan/prepare.go` needed a follow-on signature cleanup because removing the dead exec-mode read made the runtime-config parameter unused.
  Verified with focused package tests and a clean `make verify`; `internal/core/model/model_test.go` now asserts that `RuntimeConfig` omits the dead field while `model.Job` keeps the supported system-prompt surface.
