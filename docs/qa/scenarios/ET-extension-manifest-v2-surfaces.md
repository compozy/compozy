---
id: ET-extension-manifest-v2-surfaces
area: ET
title: Generate and expose one valid manifest v2 contract
persona: Ada
journey: J-extension-policy-admin
expected: Code-first registrations generate a byte-stable manifest v2 whose closed `capabilities.provides` and `permissions.requires`, extension hook source, command groups, tool command metadata, trusted_workspace, and invocation_id survive every owning projection; unknown values and malformed command metadata fail build and load before mutation.
entry_points: `compozy extension build`; `compozy extension validate`; generated `extension.toml` `capabilities.provides`/`permissions.requires`/`resources.hooks`/`resources.command_groups`/`resources.tools.command`; `compozy hooks list`; `compozy extension commands`; `GET /api/extensions/commands` (HTTP+UDS); `compozy__extensions_build|validate`; Go `extensiontest`; TypeScript `@compozy/extension-sdk/testing`; https://compozy.com/runtime/core/extensions/manifest; https://compozy.com/runtime/core/extensions/permissions; https://compozy.com/runtime/core/extensions/commands
qa_status: blocked-verify
bug_ids: BUG-20260729-public-extension-sdks-unpublished;BUG-20260807-extension-template-help
fix_status: partial
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/validate-review-kit.json; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/validate-duplicate-layouts.json;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/41-extension-template-discovery.json
last_report: docs/qa/reports/2026-08-07-remote-gateway.md
overlaps: ET-extension-code-first-authoring; ET-discover-extension-command-tree; ET-044
---

QA impact 2026-07-29: derived from the ext-improvs hard-cut contract. This scenario owns the
cross-surface declaration invariant; behavior-specific execution remains in the dev, hook, and
command scenarios.

QA impact 2026-08-02: duplicate layout diagnostics now report both owning paths; re-walk build and
validation failure output across CLI and native tools.

QA impact 2026-08-06: the closed `capabilities.provides` set now includes the public
`connectivity.provider` surface. Flag only; Tasks 08–09 own the re-walk.

QA walk 2026-08-07: the public provider contract is discoverable and the templates scaffold, but
the external SDK cannot produce the new manifest surface yet. Existing non-provider manifest-v2
evidence remains valid; the connectivity-provider projection remains blocked.
