---
status: resolved
file: internal/cli/extensions_bootstrap.go
line: 29
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVO2,comment:PRRC_kwDORy7nkc621Vam
---

# Issue 009: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Bootstrap discovery is dropping user scope and enablement state.**

This path only sets `WorkspaceRoot`, while the rest of the CLI discovery flow also passes the home dir and enablement store. As written, declarative providers/skill packs from enabled user extensions won't participate here, and bootstrap behavior can diverge from what `compozy ext ...` reports.


Based on learnings, "Keep execution paths deterministic and observable."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/extensions_bootstrap.go` around lines 26 - 29, The bootstrap
call instantiates extensions.Discovery with only WorkspaceRoot, which drops user
scope and enablement state; update the construction of extensions.Discovery used
before calling Discover(ctx) to include the same fields used by the main CLI
discovery flow (e.g., HomeDir and the EnablementStore/enablement backend from
cfg) so that extensions.Discovery{WorkspaceRoot: cfg.WorkspaceRoot, HomeDir:
cfg.HomeDir, EnablementStore: cfg.EnablementStore} (or similarly named cfg
fields) is passed into Discover; keep the existing error wrapping for
Discover(ctx) intact so bootstrap discovery honors user-enabled declarative
providers/skill packs and stays consistent with `compozy ext ...`.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d791b4d1-a09d-47ff-aca9-2faf6e21ecb7 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `INVALID`
- Notes:
  - The claimed behavior gap is stale in the current implementation. `extensions.Discovery{WorkspaceRoot: cfg.WorkspaceRoot}.Discover(ctx)` still resolves an enablement store internally through `NewEnablementStore`, and when `HomeDir` is empty that store falls back to `os.UserHomeDir()`.
  - Because discovery uses the resolved store's home directory, bootstrap discovery still includes user scope and local enablement state; it is not limited to workspace-only results.
  - No correctness issue remains to fix in `internal/cli/extensions_bootstrap.go` for this review item.
