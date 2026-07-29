---
id: ET-extension-manifest-v2-surfaces
area: ET
title: Generate and expose one valid manifest v2 contract
persona: Ada
journey: J-extension-policy-admin
expected: Code-first registrations generate a byte-stable manifest v2 whose closed `capabilities.provides` and `permissions.requires`, extension hook source, command groups, tool command metadata, trusted_workspace, and invocation_id survive every owning projection; unknown values and malformed command metadata fail build and load before mutation.
entry_points: `compozy extension build`; `compozy extension validate`; generated `extension.toml` `capabilities.provides`/`permissions.requires`/`resources.hooks`/`resources.command_groups`/`resources.tools.command`; `compozy hooks list`; `compozy extension commands`; `GET /api/extensions/commands` (HTTP+UDS); `compozy__extensions_build|validate`; Go `extensiontest`; TypeScript `@compozy/extension-sdk/testing`; https://compozy.com/runtime/core/extensions/manifest; https://compozy.com/runtime/core/extensions/permissions; https://compozy.com/runtime/core/extensions/commands
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-extension-code-first-authoring; ET-discover-extension-command-tree; ET-044
---

QA impact 2026-07-29: derived from the ext-improvs hard-cut contract. This scenario owns the
cross-surface declaration invariant; behavior-specific execution remains in the dev, hook, and
command scenarios.
