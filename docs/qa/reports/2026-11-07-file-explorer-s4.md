# File explorer S4 frontend evidence

Scope: S4 of `.compozy/tasks/file-explorer/_slices.md` on `feat/file-explorer-s4` (base
`feat/file-explorer` at `3e6b67f74`, S1 routes + S3 primitives). Frontend only: no wire, schema,
config, hook, native tool, or CLI change. The owning cross-surface audit is the spec's
`_impact-entry.md`; S10 owns docs, the official skill, and QA scenarios. Affected scenarios:
`APP-open-transcript-paths-in-editor` (plain click now opens the viewer; the hover button is
retired) and the new `ET-session-review-files`.

## Implementation

- `web/src/systems/files` (new system): adapters for the four S1 file routes (session and
  workspace roots, typed `FilesApiError` with the daemon code, `AbortSignal`), query keys and
  options (`fileEntriesOptions`, `fileContentOptions`; revealed reads are never cached), the lazy
  tree bridge `useFileTreeBridge` (one cached listing per expanded directory; the open file's
  ancestors load with it), viewer target and state models, `FilesExplorer` (tree + viewer split),
  `FileViewerPane` (select, loading, text, line jump, past end, truncated, image, image too large,
  binary, sensitive gate, outside this project, not found, denied, not a file, error), and the
  `OpenInButton` slot (absent without `files.listOpenTargets`).
- Session window: the right-side region moved from `session-window-content.tsx` into
  `session-window-right-panel.tsx` (`SessionWindowRightRegion`): a resizable Review panel
  (default 45%, min 360 px, max 75%) beside the session pane, or the Inspector. Opening one closes
  the other at the store layer. `SessionReviewPanel` has the Files tab; the toggle sits before the
  Inspector toggle and is absent in user scope.
- `useSessionReviewPanelState`: per-session open, tab, and width persisted under
  `compozy:session:review-panel:v1`; the viewer target is transient.
- Transcript links: `file-path-open-in-editor.tsx` → `file-path-link.tsx` (hard cut,
  `file-path-opener-context.ts` → `file-path-links-context.ts`, `use-file-path-open-action.ts`
  removed). The path itself is a `<button>`: a plain click opens the viewer on every client with a
  session root; on desktop Cmd/Ctrl-click still opens the configured editor (#1167). The hover
  "Open in editor" button is gone. User-scope sessions render plain code on the web.
- `resolveRootRelative` in `file-path-links.ts` (root-relative resolver mode) and
  `openFilePathInViewer` (last click wins).

## Test ownership and mapping

| ID | Suite / case |
| --- | --- |
| UT-044 | `web/src/systems/session/hooks/__tests__/use-session-topbar-slot.test.tsx`: Should offer the Review panel toggle only for a session with a file root (UT-044) |
| UT-048 | `web/src/systems/session/lib/__tests__/file-path-links.test.ts`: resolveRootRelative › Should resolve a relative token with its line inside the root |
| UT-049 | same suite: Should report absolute paths outside the root and resolve home paths under it |
| UT-050 | same suite: Should never climb above the root with dot segments |
| UT-051 | same suite: openFilePathInViewer › Should open only the second of two quick clicks |
| UT-054 | `web/src/systems/session/components/__tests__/message-markdown.test.tsx`: Should render plain code outside assistant prose and for a user-scope session (UT-044, UT-054) |
| UT-055 | `web/src/systems/files/components/__tests__/file-viewer-pane.test.tsx`: Should render no Open in control without the desktop target bridge |
| E2E-001 | `web/e2e/__tests__/files-explorer.spec.ts`: E2E-001 (fixture `internal/testutil/acpmock/testdata/browser_files_explorer_fixture.json`) |
| E2E-003 | same spec: E2E-003 |

The #1167 cases in `message-markdown.test.tsx` (UT-040..UT-046, UT-048 of that spec) were
updated to the new contract, not weakened: plain click → viewer (UT-042), keyboard on the link
itself (UT-043), modifier click → editor unchanged. `FileViewerPane` state cases live in the new
system's first suite. Desktop `shell.spec.ts` E2E-001 (#1167) now names the link "Open fixture.txt
in viewer"; its Cmd-click assertion is unchanged.

## Verification

From the repository root:

```sh
bunx turbo run typecheck --filter=compozy-web
cd web && bunx vitest run src/systems/files src/systems/session/lib/__tests__/file-path-links.test.ts \
  src/systems/session/components/__tests__/message-markdown.test.tsx \
  src/systems/session/hooks/__tests__/use-session-topbar-slot.test.tsx
bunx oxlint --deny-warnings <changed files>
bunx oxfmt --check <changed files>
cd web && bunx playwright test e2e/__tests__/files-explorer.spec.ts
```

Results are recorded in the S4 delivery report. The focused Playwright run could not complete in
this environment: the machine's data volume was full (`No space left on device` while the
harness built `web/dist`). E2E-001/E2E-003 and real-browser verification remain owed by CI or
the next run with free disk.

## Integration notes and residuals

- A directory with `contains_changes` shows Pierre's dot only once a changed descendant is loaded;
  `FileTreeDecoration` has no "contains changes" flag for unloaded directories (S3 primitive gap).
- Empty folders and capped listings show as quiet notes under the tree (`FileTreeNotes`), since
  `FileTree` has no virtual rows.
- The tree row context menu (Copy path, Open in, Reveal in Finder) is not wired: no menu
  primitive renders inside Pierre's `renderContextMenu` yet; Copy path is in the viewer header.
- Image dimensions are not shown; `FileImagePreview` exposes no natural size.
