# QA Run Report — 2026-09-10 — Layout settings Save

- Scope: issue #593; recoverable Save, zero spacing, application receipts, and shortcut preservation.
- Cadence: targeted; owning scenario `MS-configure-window-manager`, shared Save bar slice of `ET-web-ui-resilience`.
- Baseline: `e76cb83d31598d68f9a5fabdf760e1e168b13496`.
- Status: targeted real app walk and local delivery gates passed.
- Local verification verdict: PASS.
- Persona: Bruno, Studio workspace, Chromium on macOS, English UI.
- Lab manifest: `/private/tmp/compozy-qa-593/compozy-issue-593-layout-settings-save-20260910-160728-799340-lab/qa-artifacts/qa/bootstrap-manifest.json`.
- Evidence root: `/private/tmp/compozy-qa-593/` (local artifacts, no production data).

## Session matrix

| Journey | Result | Evidence |
| --- | --- | --- |
| Original failure | Reproduced: aborting PATCH disables Save and Discard; another edit retains the error | `before-failed-save.png`, `before-failed-save.har` |
| Transport recovery | Original draft retry succeeds without editing; subsequent edit clears the error, and discard restores persisted zero | `after-retryable-error.png`, `after-real-saves.har` |
| Zero spacing | Inner zero with nonzero outer insets; all outer zeros with inner 1; all gaps zero; save and reload each | `outer-zero-applied.png`, `all-zero-applied.png`, `zero-after-reload.png` |
| Rendered geometry | At zero gap, tiles meet at x=640; inner 1 moves the second tile to x=641. Top inset 1 moves y=44 to y=45. All-zero reload restores x=0/640 and y=44 | `zero-tiled-layout.png`, `zero-after-reload.png`; DOM bounding rectangles |
| HTTP failure | Temporarily removing write permission on the isolated home produces HTTP 500. Visible error retains draft and offers retry/discard. Permissions restored in `finally`; retry succeeds | `server-write-error.png`, `after-real-saves.har` |
| Shortcut integrity | Change shortcut/global shortcut maps and alias via public PATCH while spacing draft is open; Save preserves their newer values | `shortcut-live-edit.json`, `all-zero-shortcuts-preserved.json` |
| Repeated save | Identical public PATCH reports skipped/no changes without advancing generation | `repeated-save.json` |

## Root cause and application evidence

The failed TanStack mutation error outlived the editor error reset and overrode dirty state. The Save
bar then disabled both recovery actions. The frontend full-config serializer also omitted global
shortcuts, and replacing the full config could erase them or overwrite separately edited shortcuts.

Both the Layouts adapter and the backend section-echo handler discarded application outcome. The
fixed real PATCH returns the original section plus `apply`. One captured retry returned HTTP 200,
`applied: true`, `next_action: none`, and record `cfgapp-f748c8967ffcfc30`. The UI displayed
“Settings saved and applied.” Zero was valid before the fix; no validator relaxation was required.
The earlier connection-refused logs do not prove a daemon crash.

The live walk used the real daemon and Web app. Transport abortion and filesystem permission failure
were deliberate disruptions. No application receipt was mocked in the browser. Partial application
failure, warnings and restart/new-session result interpretation are verified at the owning adapter
and handler test boundaries, not claimed as live platform failures.

## Change impact

- Native tools: no IDs, toolsets, permissions or CLI commands changed. HTTP/UDS share the settings
  handler; PATCH adds an `apply` receipt and optional `preserve_shortcuts` flag. OpenAPI and generated
  Web contracts co-ship. Existing section response fields remain available.
- Extensibility/config: no new persisted config keys, hooks or extension contracts. The optional
  request flag preserves current shortcut maps inside the existing settings mutation lock; explicit
  top-level maps retain replacement semantics. Omitting the flag preserves legacy full replacement.
- User state/isolation: no schema migration or data deletion. The existing user-scoped settings
  boundary remains; workspace layouts and profile routing are unchanged. The lab used its own home,
  socket, ports and workspace. No host daemon restart or production settings write occurred.
- Web/Docs: Layouts editor, shared Save bar recovery and settings adapter changed. Existing shortcut
  consumers retain their section parsing. Site docs, official `skills/compozy` window-management
  reference, and the two affected QA scenario entries were updated.

## Verification and limits

- Root `bunx turbo run test typecheck build --filter=./web` passed: 771 suites / 7,133 tests,
  typecheck and build. The final gate passed 771 suites / 7,135 tests.
- `CGO_ENABLED=1 go test -race ./internal/settings ./internal/api/core` passed; the receipt case also
  passed after extending it to both HTTP and UDS handler shims.
- Root `bunx turbo run build --filter=./packages/site` passed, including TypeScript and static pages.
- `make gate` passed on the final production diff: codegen-check, Go lint (zero issues), scoped
  Go race tests, and Web lint (zero warnings/errors), typecheck and all tests. Required CI is
  tracked on the current PR head.
- Laboratory teardown completed with `clean: true`.

The final `bunx turbo run build --filter=./web` also passed after the pending-action adjustment.

The Web build emitted existing CSS `::highlight` optimizer and bundle-size warnings. The full Web
suite also emitted unrelated harness warnings. No warning suppression was added. The Go convention
checker produced identical findings on the base and changed test files; there are no new findings.

This targeted walk does not claim Electron global-hotkey registration, Linux, provider-backed agent
workflows, or the full resilience matrix. The application-rejection test is a unit I/O boundary test;
the live failure evidence is transport and HTTP write rejection.
