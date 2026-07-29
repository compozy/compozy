---
id: ET-extension-dev-reload-loop
area: ET
title: Iterate on a dev-linked extension without reinstalling
persona: Bruno
journey:
expected: `compozy extension dev` links the built generation to the current workspace with no trust prompt, an edit plus `reload` (or `--watch`) makes the next invocation return the new behavior while other workspaces keep serving the published build, `logs --follow` streams the redacted per-instance ring, and removing the dev instance restores the published one.
entry_points: `compozy extension dev [dir] --watch`; `compozy extension reload <name>`; `compozy extension logs <name> --follow|--global`; `compozy extension remove <name>`; `POST /api/extensions/dev`; `POST /api/extensions/{name}/reload`; `GET /api/extensions/{name}/logs?follow=1`; `DELETE /api/extensions/{name}`; `compozy__extensions_dev|reload|logs|remove`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-extension-code-first-authoring; ET-020; ET-022
---

Added by ext-improvs Task 04 (Phase D dev lane). Planning flag only; no QA session ran.

Inner loop: walk one origin through link, invoke, edit, reload, observe on a stamped binary and prove the
changed behavior comes from the new `dist/gen-<hash>` generation rather than a restart side effect. Zero
trust ceremony — no `allow_unverified` prompt, and `dev` mints no install row.

Instance isolation: the overlay is keyed by (name, workspace), the published row is never displaced, a
second workspace keeps serving the published build, and agent-facing list/status/logs projections stay
workspace-filtered. `--global` logs are operator-only.

Failure lane: reload a deliberately broken generation and assert the last-good generation keeps serving
with status `errored (activation_failed)` instead of taking the extension down; a hostile or unknown
`generation_hash` returns 400 and a reload without a link returns 409.

Logs: secrets redacted at ingestion across ring, one-shot read, and SSE alike; the bounded ring drops
oldest without blocking; `--follow` delivers the named `extension_log` SSE event, resumable via `after`.

Restore: `remove` inside the workspace unlinks only the overlay, emits `extension.dev.unlinked`, and the
published instance serves again on the next invocation; the same verb outside a workspace scope still owns
published removal (ET-020).

Surfaces: `internal/cli/extension_dev.go`, `internal/api/core/extensions_dev.go`,
`internal/daemon/extensions_dev.go`, `internal/extension/manager_dev_lifecycle.go`; payload additions
`overrides_published` and `origin_path`.

Dedup: ET-015..ET-023 keep the install/update/remove/status lifecycle and were already `untested`, so no
verdict needed resetting; this file owns the dev overlay loop only. No journey owns the dev inner loop yet
— the next planning cycle should map it and link it here.
