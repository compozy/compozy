---
title: Project configuration cannot grant MCP consent to itself
type: fix
---

Security change: workspace and workspace-profile configuration can only restrict inherited external
tool policy. `tools.policy.external_default` keeps the stricter value (`disabled` before `ask` before
`enabled`), and `trusted_sources` retains only entries already trusted by the preceding layer.
Omitting the trust list inherits it; an empty list removes all trust. Attempted project grants are
ignored with one warning per daemon process. Put intentional grants in user or personal profile
config. Project files and existing explicit server approvals are preserved.
