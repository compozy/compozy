# Outside-of-diff from Comment 4

**File:** `internal/core/workspace/config.go`
**Date:** 2026-04-05 19:22:20 America/Sao_Paulo
**Status:** - [x] RESOLVED

- Disposition: VALID

- Rationale: `ProjectConfig.Validate()` currently validates `[exec].output_format` in isolation, but CLI/runtime application merges `[defaults]` into `[exec]`. That means `[defaults] output_format = "json"` with `[exec] tui = true` can pass config loading even though the effective exec config is invalid.

## Details

<details>
> <summary>internal/core/workspace/config.go (1)</summary><blockquote>
> 
> `169-188`: _⚠️ Potential issue_ | _🟠 Major_
> 
> **Validate the merged exec output format, not only `[exec].output_format`.**
> 
> This misses configs like `[defaults] output_format = "json"` with `[exec] tui = true`. `ProjectConfig.Validate()` accepts that file, but the effective exec config is still invalid and later fails at runtime once defaults are applied. Fold `defaults.output_format` into this check when the exec override is unset.
> 
> 
> 
> Also applies to: 236-247
> 
> <details>
> <summary>🤖 Prompt for AI Agents</summary>
> 
> ```
> Verify each finding against the current code and only fix it if needed.
> 
> In `@internal/core/workspace/config.go` around lines 169 - 188,
> ProjectConfig.Validate currently only validates validateExec(cfg.Exec) and
> misses cases where output_format is set in Defaults but Exec.output_format is
> unset (e.g., defaults.output_format="json" with exec.tui=true), causing invalid
> effective exec config at runtime; update ProjectConfig.Validate to compute the
> effective exec config by merging cfg.Defaults (specifically
> Defaults.OutputFormat) into cfg.Exec when Exec.OutputFormat is empty and then
> pass that merged exec struct into validateExec (also apply the same merge logic
> in the other validation block referenced around validateExec usage at the later
> location mentioned).
> ```
> 
> </details>
> 
> </blockquote></details>
