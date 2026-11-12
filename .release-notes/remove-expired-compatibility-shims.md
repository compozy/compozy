---
title: Remove expired compatibility shims
type: fix
---

Removed the compatibility shims scheduled for v0.6.1 after their final v0.6.0 migration window.

Migration notes: run v0.6.0 before upgrading if persisted retired agent-kind files/config tables or retired
memory/compaction settings still need migration. Resolve deferred writes there, and remove retired
SOUL, hook, manifest and automation declarations. v0.6.1 rejects these declarations
without rewriting their files. Existing inactive archives, SQL migrations, memory files and permanent
session-catalog recovery remain. Bundled `dev-cycle` state no longer transfers automatically to
`spec-cycle`; retained state stays available for operator migration and doctor diagnostics.
Remove `permissions.provider_full_access` and use `permissions.mode`. ACP adapters must send
`cachedReadTokens` / `cachedWriteTokens`; removed aliases no longer contribute cache counts.
Take-over no longer translates pre-v0.4.8 canceled-launch process failures; recover those sessions
with v0.6.0 first. Marketplace source management remains experimental.

Retired toolsets fail catalog validation. Exact retired tool IDs remain inert policy atoms that
match no tool; explicit allow-lists remain enforced. Remove these references from authored policies.
