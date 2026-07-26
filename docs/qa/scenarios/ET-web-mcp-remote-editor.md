---
id: ET-web-mcp-remote-editor
area: ET
title: MCP remote-capable editor (stdio/http/sse) with mirrored validation
persona: Bruno
journey: J-mcp-authorize-repair
expected: The editor switches field sets by transport. stdio keeps command, ordered args, non-secret env, and hybrid secret_env bindings; remote (http/sse) keeps url + optional OAuth (client_id + metadata triad + scopes + client-secret binding) and omits command/args/env/secret_env. Switching transport clears hidden fields so the new form remains completable. Existing plain env and secret bindings render as presence-only configured state; unchanged exact-target fields use `preserve_env` or `preserve_secrets`, while renames and scope changes require explicit replacement values. Vault inventory renders distinct loading, error/retry, ready-empty, and ready-with-refs states. Scope lives in `validateSearch`; an initial valid `workspace_id` is adopted once, then sidebar selection owns the workspace and updates the URL.
entry_points: web `/marketplace/mcps?tab=installed` Add MCP server/Edit configuration; `PUT /api/settings/mcp-servers/{name}`
qa_status: untested
bug_ids: BUG-20260715-mcp-editor-vault-ref-case
fix_status: BUG-20260715-mcp-editor-vault-ref-case fixed
retest_status: pending transport-switch and incomplete-secret-binding regression
fix_commits:
evidence: web/src/systems/settings/lib/mcp-editor-model.ts; web/src/systems/settings/components/mcp-server-editor.tsx; .compozy/tasks/marketplace/evidence/visual/task-08/editor-http-desktop; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-editor-vault-ref-case.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/mcp-editor-vault-ref-case-green.png
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: MS-029
---

story: As a builder I edit any server class the marketplace can install (local stdio or remote http/sse with OAuth) directly in the product with validation that mirrors the daemon.

src: docs/design/opendesign/mcp-management.html

inventory: Needs QA

QA impact 2026-07-15: new behavior from Task 08 (ADR-006). Flagged untested for the next QA cycle.

QA impact 2026-07-16: reset after hardening editor draft transitions and stdio secret completeness. Retest create and edit flows in both directions (stdio ↔ http/sse), confirm hidden fields cannot deadlock Save or leak into the request, and verify a named secret row without a value/ref visibly blocks Save while existing refs remain valid and secret values remain write-only.

QA impact 2026-07-17: existing refs are no longer delivered to or rendered by the editor. Retest the
"Keep configured" path for environment and OAuth secrets and inspect requests for `preserve_secrets`
without binding refs in the preceding GET response or visible UI.

QA impact 2026-07-17: plain environment values are now presence-only through `env_keys`. Retest
same-key `preserve_env`, rename and cross-scope replacement requirements, all Vault inventory
states, one-shot workspace deep-link adoption, and subsequent sidebar-to-URL synchronization.

QA impact 2026-07-18: the generic editor moved into Marketplace Installed. Retest custom stdio,
http, and sse creation plus exact-scope editing after the legacy `/mcp` route hard cut.
