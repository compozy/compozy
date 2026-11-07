---
id: ET-files-search
area: ET
title: Go to file, search file contents, and find in the open file
persona: Dora
journey: J-14
expected: In any files explorer (Review panel Files tab or Files app), Cmd/Ctrl+P opens Go to file with daemon-ranked fuzzy matches and `name:line`; Cmd/Ctrl+Shift+F opens the Search pane with grouped results, options, globs, a sensitive-files note and an inline invalid-regex error; a match opens the viewer at its line with Find in file seeded; Cmd/Ctrl+F finds over the whole loaded file only while focus is in the viewer.
entry_points: files explorer Go to file button and Cmd/Ctrl+P; Files ↔ Search pane switch and Cmd/Ctrl+Shift+F; viewer Cmd/Ctrl+F find bar
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: lab compozy-file-explorer-integration-20261107-073905-206925 (qa-artifacts/qa/journey-log.jsonl, screenshots/S01–S08, notes/api-edges.txt)
last_report: docs/qa/reports/2026-11-07-file-explorer-integration.md
overlaps: ET-files-app; ET-session-review-files
---

Introduced by the file-explorer spec (US-013–US-015; `_uiux.md` S11–S13). Walk it in a Files window
(the Review panel's Files tab hosts the same explorer, covered by E2E-009–E2E-011) on a git project with a long file holding one unique word deep
inside, a file with repeated matches, a `.env` and a public `.env.example`.

Walk:

1. Focus the explorer and press Cmd/Ctrl+P. Type a fuzzy query (`lngtxt`): the first row is
   `src/long.txt` with highlighted characters. Enter opens it in the viewer and the tree reveals it.
2. Open Go to file again and type `main.go:12`: the footer reads "Opens at line 12". Enter opens the
   file at line 12. Escape closes the box and returns focus, so explorer shortcuts keep working.
3. Press Cmd/Ctrl+Shift+F: the Search pane opens with focus in the query. A query with matches shows
   "N results in M files", grouped by file.
4. Click a match: the viewer opens at that line, with Find in file seeded with the query and its
   position ("7 of 39").
5. Search a word that appears only in `.env` and `.env.example`: only `.env.example` is listed, and
   the status reads "1 sensitive file not searched".
6. Open the include/exclude fields and set include `src/**`: only files under `src/` are listed.
7. Turn on **Use regular expression** and type `foo(`: the field is marked invalid and the error
   shows the daemon message inline.
8. Open the long file, focus the viewer, press Cmd/Ctrl+F and type the unique word: "1 of 1",
   scrolled to its line (4800) even though that line was never rendered before.
9. With focus in the tree, Cmd/Ctrl+F does not open the viewer's find bar.

Owning automated coverage: web `use-files-search.test.tsx` (UT-091, UT-092), `search-text.test.ts`
(UT-093), `file-viewer-pane.test.tsx` (UT-094, UT-095), `@compozy/ui` files suites, and
`web/e2e/__tests__/files-explorer.spec.ts` E2E-009–E2E-011; server `TestFileSearch*` and the
`TestDaemonSearchSessionFilesWorktreeBoundIT005` search case.

## Walk 2026-11-07

Same lab and build as `ET-session-review-files` (integration `410a3566e`, fix `c5d1d54a6`), walked in
the `lab-project` Files window.

| Step | Result | Evidence |
| --- | --- | --- |
| 1 Fuzzy Go to file | pass | `lngtxt` → `src/long.txt` (`S01-go-to-file.png`) |
| 2 `name:line` | pass | `main.go:12` opened at line 12 |
| 3 Search pane | pass | "39 results in 1 file" for `TODO marker` (`S03-search.png`) |
| 4 Match → viewer with find | pass | Line 7, find "TODO marker", "7 of 39" (`S04-search-open-match.png`) |
| 5 Sensitive note | pass | Only `.env.example`; "1 sensitive file not searched" (`S05-sensitive-note.png`) |
| 6 Include glob | pass | `src/**` → "117 results in 2 files", all under `src/` (`S06-globs.png`) |
| 7 Invalid regex | pass | Inline: "projectfiles: invalid query: error parsing regexp: missing closing ): `foo(`" (`S07-invalid-regex.png`) |
| 8 Find in file deep line | pass | "1 of 1", active line 4800 (`S08-find-in-file.png`) |
| 9 Cmd/Ctrl+F scope | pass | No find bar with focus in the tree |

The invalid-regex message is the daemon's raw text (S9 residual); the API returns the same message
(`notes/api-edges.txt`).
