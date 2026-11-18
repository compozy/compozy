# File explorer S9 frontend evidence

Scope: S9 of `.compozy/tasks/file-explorer/_slices.md` on `feat/file-explorer-s9`: Go to file,
the explorer's Files ↔ Search pane, and Find in file (`_uiux.md` S11–S13; US-013–US-015).
Frontend only. It consumes S8's generated `findSessionFiles`/`findFiles` and
`searchSessionFileContents`/`searchFileContents` DTOs. No backend, schema, config, hook, native
tool, or CLI changes. The spec's `_impact-entry.md` owns the cross-surface audit; S10 owns site
docs and the `ET-files-search` scenario. Affected scenarios: `ET-files-search` (new in S10) and
`ET-session-review-files` (the explorer's left pane gains the Files/Search switch).

## Implementation

- `web/src/systems/files`:
  - adapters `findFiles` and `searchFileContents` (session and workspace forms; globs travel as
    repeated `include`/`exclude`; typed `FilesApiError` codes) and `fileFindOptions` /
    `fileSearchOptions`, one query key per request;
  - `GoToFileBox` (S11) composes `Command`/`CommandInput`/`CommandList`/`CommandItem`, anchored to
    the explorer, with recent files (empty query), daemon-ranked rows with highlighted byte ranges,
    folder reveal, lock badge, `name:42` line hint, index/truncation footer, and unavailable state;
  - `FilesSearchPane` (S12): query with Match case / Whole word / Regex toggles, collapsible
    include/exclude globs, status line (idle, under 2 characters, progress, summary, no results
    with active filters, cap, timeout, sensitive exclusions, walk engine), inline invalid-regex
    error, and the virtualized `FilesSearchResults` (TanStack Virtual; file groups collapse;
    ↑/↓ step between matches);
  - `ViewerFindBar` (S13) inside `OpenFileView`: literal find over the loaded text's line model,
    "3 of 17", Enter/↓ next, Shift+Enter/↑ previous with wrap-around, Match case, Escape closes
    and keeps the scroll, "Searching the first 1 MB" on truncated files, pre-filled from Search;
  - hooks `use-files-find`, `use-files-search` (300 ms debounce, Enter runs at once, replaced
    requests abort), `use-viewer-find`, `use-files-explorer`, `use-files-search-results`, and the
    persisted per-root `use-files-search-prefs` (XState Store: recent files ≤10, last 10 queries,
    options);
  - scoped shortcuts (`lib/files-shortcuts.ts`): Cmd/Ctrl+P and Cmd/Ctrl+Shift+F fire only while
    focus is in the explorer, Cmd/Ctrl+F only while focus is in the viewer.
- Byte offsets: S8 ranges are UTF-8 byte offsets; `byteRangesToCharRanges` converts them to UTF-16
  and clamps them into the excerpt. Ellipses render outside the excerpt text, so ranges never
  shift.
- `@compozy/ui`:
  - `CodeViewer` gains `findMatches`/`activeFindMatch`. Matches paint with the CSS Custom
    Highlight API over Pierre's rendered `[data-line]` text (repainted on render and scroll); the
    active match scrolls into view through `CodeView.scrollTo`, even if its line was never
    rendered. Story "Find matches".
  - `FileTree`: a controlled `selectedPath` directory (`dir/`) is expanded as well as revealed
    (Go to file on a folder, US-013.AC-4).
  - icons `CaseSensitive`, `WholeWord`, `Regex`.
- `FilesExplorer.onOpenFile` now carries `(path, line?, find?)`; `SessionReviewPanel` forwards it
  through `fileViewerTarget(path, line, find)`.

## Test ownership and mapping

| ID | Suite / test |
| --- | --- |
| UT-091 | `web/src/systems/files/hooks/__tests__/use-files-search.test.tsx` › useFilesFind › Should render only the newest query when answers arrive in reverse order |
| UT-092 | same file › useFilesSearch › Should skip short queries, debounce typing, run Enter at once, and abort replaced requests |
| UT-093 | `web/src/systems/files/lib/__tests__/search-text.test.ts` › Should clamp ranges inside a minified line's excerpt and mark both ellipses |
| UT-094 | `web/src/systems/files/components/__tests__/file-viewer-pane.test.tsx` › Find in file › Should count matches over the full text and scroll an unrendered match into view |
| UT-095 | same file › Find in file › Should leave Cmd+F to the browser while the viewer is not focused |
| E2E-009 | `web/e2e/__tests__/files-explorer.spec.ts` › E2E-009: Go to file opens a fuzzy match and a name:line target |
| E2E-010 | same file › E2E-010: Search finds content, opens a match at its line, and reports a bad regex |
| E2E-011 | same file › E2E-011: Find in file reaches a match deep in a long file |

Supporting coverage in the owning suites: byte-range conversion, indentation trimming, literal
find and line suffix parsing (`search-text.test.ts`); repeated globs and the inline invalid-regex
state (`use-files-search.test.tsx`); Search pre-fill, truncation note, and the no-results state
(`file-viewer-pane.test.tsx`); `CodeViewer` active-match scroll and `FileTree` directory reveal
(`packages/ui/src/components/files/__tests__/`).

## Verification

From the repository root unless noted:

```sh
bunx turbo run typecheck --filter=./web --filter=./packages/ui
cd web && bunx vitest run src/systems/files
cd packages/ui && bunx vitest run src/components/files src/components/__tests__/input-group.test.tsx
cd web && bunx oxlint --deny-warnings <changed files> && bunx oxfmt --check <changed files>
cd packages/ui && bunx oxlint --deny-warnings <changed files> && bunx oxfmt --check <changed files>
cd web && bunx playwright test e2e/__tests__/files-explorer.spec.ts -g 'E2E-009|E2E-010|E2E-011'
```

Results: typecheck 5/5 tasks; web files suites 19/19; `@compozy/ui` files and input-group suites
24/24; lint and format clean; E2E-009, E2E-010 and E2E-011 passed (1.2 min, daemon-served build).

The Playwright harness must run without the managed session's `COMPOZY_*` environment. Inside a
CompozyOS session, `COMPOZY_INTERNAL_RESTART_OPERATION_ID` leaks into the harness daemon, which
exits with `load restart operation … not found` before readiness. Unset `COMPOZY_*` first.

Real-browser check: a throwaway Playwright walk (not committed) on the same daemon-served build,
1600×950, light and dark (`compozy.theme`), captured Go to file (recent, results, no match,
`:line` hint), Search (idle, under 2 characters, results with globs, no results with the
sensitive and walk notes, invalid regex), and Find in file (matches, a match on line 4,800, no
results, truncated file with the 10,000+ cap). The walk found and fixed two defects:

- Escape from Go to file left focus on `body`, so explorer shortcuts stopped working. The box
  now returns focus to where it was opened (covered in E2E-009).
- `InputGroupInput` kept `Input`'s `focus-visible:shadow-focus-ring` (`ring-0` doesn't clear a
  shadow), drawing a square ring inside the rounded group. The ring now sits on the group for
  `input` controls; the composer's textarea variant is unchanged.

## Deviations and residuals

- Match highlights in the viewer use the CSS Custom Highlight API. Its `::highlight()` rules
  must live in Pierre's shadow root, so `CodeViewer` passes them through `unsafeCSS`, using only
  token variables (`--color-brand-tint`, `--color-brand-tint-strong`, `--color-brand`). The design
  constraint reserved `unsafeCSS` for the focus ring; this is the second use. Engines without the
  API still find, count, and scroll, but paint no ranges.
- Match marks use the brand tint, because `accent` is reserved for the hover wash in
  `@compozy/ui`.
- The active match is underlined in brand, since `::highlight()` can't draw outlines.
- The trailing excerpt ellipsis is inferred: the response has no full-line length, so it shows
  when the excerpt reaches 300 characters.
- Find in file is literal. A regex search seeds it with the text the regex matched, with Match
  case on.
- ↑/↓ in the Search query walk the last 10 queries. ↑/↓ in the result list step between matches
  and open each one.
- S8's invalid-regex message reads ``projectfiles: invalid query: error parsing regexp: missing
  closing ): `foo(` ``, not `_dx.md`'s `invalid search: missing ) in "foo("`. The UI shows the
  daemon message verbatim; the wording belongs to S8.
- The pre-merge base `818ba3d5d` failed E2E-001 (Inspector overlay) and the "No QueryClient set"
  session tests. Both are fixed on `feat/file-explorer`, and both pass after the merge below.

## Integration merge (`feat/file-explorer` at `83de85207`)

`git merge feat/file-explorer` produced five conflicts. Each was resolved so that both sides'
features survive:

- `files-explorer.tsx`: S9's structure (pane switch, Search pane, Go to file,
  `useFilesExplorer`, `onOpenFile(path, line?, find?)`) with S6's `unavailableAction` (passed to
  `FileTreeUnavailableState`) and `onOpenInFilesWindow` (the tree-header action, given the ready
  root and the open file's path).
- `query-options.ts`: S7's `openTargetsOptions` plus S9's `fileFindOptions`/`fileSearchOptions`.
- `session-review-panel.tsx`: S5's Changes/Files tabs and S6's Files-window action, with the
  explorer forwarding `fileViewerTarget(path, line, find)`.
- `file-viewer-pane.test.tsx`: S7's Open in cases plus S9's Find in file block.
- `files-explorer.spec.ts`: one suite with E2E-001/002/003/009/010/011.
- Merge-time fix: the Files app's `openFile` (`use-files-app-window.ts`, S6) now forwards `line`
  and the Find seed, so Go to file `name:42` and Search matches work in the Files window too.
- Follow-up commit: E2E-003's "no Open in control in the browser" assertion matched S6's
  "Open in Files window" header action, which is a browser-available navigation. The assertion
  now targets S7's controls (`/^Open in (?!Files window$)/` and "Choose application"), so the
  invariant is unchanged. This failure exists on `feat/file-explorer` itself.

Verification on the merged tree, with every `COMPOZY_*` variable unset for Playwright:

```sh
bunx turbo run typecheck --filter=./web --filter=./packages/ui
cd web && bunx vitest run src/systems/os src/systems/files src/systems/session
cd packages/ui && bunx vitest run src/components/files src/components/__tests__/input-group.test.tsx
cd web && bunx oxlint --deny-warnings src/systems/files src/systems/session/components/session-review-panel.tsx e2e/__tests__/files-explorer.spec.ts
cd web && bunx oxfmt --check src/systems/files e2e/__tests__/files-explorer.spec.ts
cd web && bunx playwright test e2e/__tests__/files-explorer.spec.ts e2e/__tests__/session-changes.spec.ts
```

Results: typecheck 5/5; Vitest 262 files / 3,598 tests passed; `@compozy/ui` 24/24; lint and
format clean; Playwright 10/10 passed (E2E-001, 002, 003, 009, 010, 011 and session-changes E2E-004–007; 2.1 min).
