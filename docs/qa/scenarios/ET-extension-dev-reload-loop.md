---
id: ET-extension-dev-reload-loop
area: ET
title: Iterate on a dev-linked extension without reinstalling
persona: Bruno
journey: J-extension-dev-lifecycle
expected: `compozy extension dev` links the built generation to the current workspace with no trust prompt, an edit plus `reload` (or `--watch`) makes the next invocation return the new behavior while other workspaces keep serving the published build, `logs --follow` streams the redacted per-instance ring, and removing the dev instance restores the published one.
entry_points: `compozy extension dev [dir] --watch`; `compozy extension reload <name>`; `compozy extension logs <name> --follow|--global`; `compozy extension remove <name>`; `POST /api/extensions/dev`; `POST /api/extensions/{name}/reload`; `GET /api/extensions/{name}/logs?follow=1`; `DELETE /api/extensions/{name}`; `compozy__extensions_dev|reload|logs|remove`
qa_status: pass
bug_ids: BUG-20260801-extension-cli-workspace-reads
fix_status: fixed
retest_status: pass
fix_commits: 98feabf
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json; /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa; /Users/pedronauck/dev/qa-labs/compozy-session-extension-boundary-20260801-140242-564687-lab/qa-artifacts/qa/boundary-verification.json; /Users/pedronauck/dev/qa-labs/compozy-session-extension-boundary-20260801-140242-564687-lab/qa-artifacts/qa/teardown.json; internal/daemon/daemon_extension_authoring_e2e_integration_test.go; internal/cli/extension_test.go; internal/cli/client_test.go; /tmp/compozy-loops-postrebase.N1iPVr/teardown.json
last_report: docs/qa/reports/2026-08-01-loops-paper-adoption.md
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

QA impact 2026-07-29: unlink now discards the workspace instance's retained log ring before a same-name
relink, and the Web confirmation no longer treats a published bundle dependency as a blocker for the
workspace-scoped unlink. Historical evidence is retained; replay unlink/relink with the published row
active and prove the new overlay starts with an empty ring.

2026-08-01 isolated boundary walk: `extension dev` accepted the public registration
`ws_fcd6ecd9076c58c6` and activated `boundary-search` under stable workspace identity
`01KYYTH06HE0TR5WV2BTHQMEWA`. CLI tool invocation returned the original behavior. After a source edit,
`extension reload` changed the generation hash from `fe63cc9a…` to `ae85d1ff…`, and the next invocation
returned the reloaded behavior. Independent HTTP list/status/log reads resolved the same stable identity;
workspace removal left zero scoped extensions. The generated Go project used a local SDK replace because
the public SDK is not published, so this does not settle `ET-extension-quickstart-verbatim`.

2026-08-01 post-rebase public replay: Bruno linked `post-rebase-probe` to workspace
`ws_44d9a2ed896b6e2e`. The rebuilt CLI listed and inspected that exact dev overlay by stable
workspace ID. Tool invocation returned the initial behavior; after a source edit and reload,
generation `c9f43a2e…` returned `Final reload result for workspace overlay`. Logs were readable,
removal unlinked only the overlay, a fresh scoped list omitted it, and scoped status returned not
found. The replay discovered and verified BUG-20260801-extension-cli-workspace-reads. Teardown
recorded zero surviving processes and listeners.
