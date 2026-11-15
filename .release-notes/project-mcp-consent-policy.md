---
title: Project configuration cannot grant MCP consent to itself
type: fix
---

Security change: `tools.policy.external_default` and `trusted_sources` are now read only from user
and personal profile configuration. Workspace and workspace-profile values are ignored with one
warning per daemon process. A cloned project cannot admit its own MCP server or grant read-only
source trust. Move intentional policy grants to your user or personal profile config; project files
and existing explicit server approvals are preserved.
