---
status: resolved
file: internal/core/extension/assets.go
line: 12
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093952670,nitpick_hash:3a59c0c5a054
review_hash: 3a59c0c5a054
source_review_id: "4093952670"
source_review_submitted_at: "2026-04-11T16:18:34Z"
---

# Issue 008: Keep DeclaredProvider as a narrower inventory type.
## Review Comment

Embedding both a cloned `ProviderEntry` and a mutable `*Manifest` makes this struct carry two sources of truth for provider data and gives downstream code access to unrelated manifest state. Passing only the launch-time context you actually need here would keep the provider extraction boundary easier to reason about.

Based on learnings: Maintain clear system boundaries and establish clear ownership of each boundary; Prioritize cohesion within boundaries over convenience and minimize coupling between systems.

## Triage

- Decision: `invalid`
- Notes:
  - `DeclaredProvider` is intentionally a launch-context bundle, not just a copied provider row. The bridge/runtime constructors need the provider entry plus the extension-level manifest, manifest path, and extension directory to preserve subprocess, capability, and source metadata.
  - There is no competing mutable source of truth in the current code path: provider fields are cloned into `ProviderEntry`, while `Manifest` remains the canonical extension-level configuration needed at runtime.
