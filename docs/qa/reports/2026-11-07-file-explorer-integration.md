# File explorer integrated QA walk (S10)

Scope: S10 of `.compozy/tasks/file-explorer/_slices.md`. This walk covers the integrated
`feat/file-explorer` build (S1–S9 at `410a3566e`) on branch `feat/file-explorer-s10`. A defect fix
was re-walked at `c5d1d54a6`. Scenarios: `ET-session-review-files`, `ET-session-review-changes`,
`ET-files-app`, `ET-files-search`, `APP-open-in-targets`, and `APP-open-transcript-paths-in-editor`.
Slice evidence: `2026-11-07-file-explorer-s1.md` … `-s9.md`. Cross-surface audit:
`docs/_memory/change-impact.md` ("Project file explorer, file viewer, and session Changes").

## Lab

- `eng-qa-bootstrap`: targeted profile, surfaces `web`, `api` and `cli`. Lab
  `compozy-file-explorer-integration-20261107-073905-206925`, with its own `COMPOZY_HOME` (short
  temp root), HTTP port 52129, its own UDS and tmux sockets, and provider usage disabled.
- Binaries: the daemon (`go build ./cmd/compozy`) and `web/dist` (`bun run build` in `web/`), both
  built from the worktree and served through `COMPOZY_WEB_DIST_DIR`. The agent was the acpmock
  driver with a fixture (`review-lab`) that cites file paths, edits `src/b.go` and creates `e.go`.
  It runs with `model: claude-sonnet-5-5` and a fixture `model` config option.
- Every `COMPOZY_*` variable was unset for the lab daemon and for the E2E harnesses, because the
  managed session leaks `COMPOZY_INTERNAL_RESTART_OPERATION_ID`. `HOME` was a fake home holding
  `.ssh/config`.
- Projects:
  - `lab-project`: git, `main` plus `feat/review` with a committed edit, uncommitted `README.md`,
    `.env` and `bun.lock`, untracked `notes.txt`, a PNG, a 50 KB binary, and a 6,000-line file.
    It has the ready worktree `feature-lab`.
  - `bulk-project`: 250 untracked files.
- Driver: Playwright Chromium (`playwright-core` 1.64.1) at 1600×1000, with scripts outside the
  repository. Results stream to the lab's `qa-artifacts/qa/journey-log.jsonl`; screenshots are in
  `qa-artifacts/qa/screenshots/`.

## Results

| Scenario | Verdict | Notes |
| --- | --- | --- |
| ET-session-review-files | pass (after fix) | 13 steps. "Open in Files window" failed (BUG-20261107-files-root-internal-workspace-id), fixed and re-walked |
| ET-session-review-changes | pass | 10 of 11 steps. PR chip not walked (no forge remote); one transient 500, not reproduced (open finding) |
| ET-files-app | pass | Dock order, root picker, one window per project, switcher, palette, project context menu, removed-worktree recovery |
| ET-files-search | pass | Go to file (fuzzy and `name:line`), Search pane, match → viewer with find seed, sensitive note, include glob, invalid regex, deep-line find, Cmd/Ctrl+F scope |
| APP-open-in-targets | partial | Packaged E2E-008 passed; real editors, Finder reveal and native confirmations need an operator walk |
| APP-open-transcript-paths-in-editor | partial | Packaged E2E-001 (#1167) passed for the viewer-first contract; steps 3–7 not re-walked |

Per-step evidence is in each scenario's "Walk 2026-11-07" section.

### CLI and API spot checks

- `compozy session changes <id>` printed "Branch · feat/review → main (merge base 198df79)", a
  status table and "6 files changed, +8 −3".
- `--scope uncommitted --patch README.md --patch .env -o json` returned README's patch and `.env`
  as `sensitive: true` with no text. `--reveal` returned the `.env` patch.
- A User-scope session exited 1 with `error: root is unavailable (root_unavailable)`.
  `--base nope-ref` exited 1 with `base_not_found`.
- HTTP:
  - `files/content?path=.env` → `kind: sensitive`; with `reveal=true` → text.
  - `.git/config` and `../outside` → `400 invalid_path`.
  - `files/find?q=maingo` → `src/main.go`.
  - `files/search?q=needle` → line 4800, `sensitive_excluded: 1`.
  - Regex `foo(` → `400 invalid_query`.
  - `changes?scope=branch&base=nope-ref` → `409 base_not_found`.

Evidence: lab `qa-artifacts/qa/notes/cli-changes-human.txt`, `cli-changes-uncommitted.json`,
`cli-api-edges.txt` and `api-edges.txt`.

### Desktop (APP-*)

```sh
bun run --cwd desktop build:e2e-update
cd desktop && bun x playwright test --config playwright.config.ts \
  --grep 'E2E-001 \(#1167\)|E2E-008: Open in' --reporter=line
# 2 passed (20.0s)
```

The packaged production app strips remote debugging, and the instrumented E2E app records editors
through fake executables. Real editor launches, Finder reveal and native confirmation sheets
therefore remain an operator walk. This is recorded as the `partial` status of both APP scenarios.

## Defects

### BUG-20261107-files-root-internal-workspace-id — fixed (`c5d1d54a6`)

File, search and changes responses, `compozy session changes -o json` and `compozy__session_changes`
set `root.workspace_id` to the project's internal identity (`01M6…`) instead of the public `ws_…` ID.
The Review panel's "Open in Files window" routes by that value, so it opened a Files window that read
"Project not found".

- Fix: `projectFileRoot` (core) and the native binding use the public registry ID.
- Regressions, both red before the fix: the existing `TestProjectFileContentReveal`, whose fixture
  already had distinct registry and identity IDs, and `TestNativeSessionChanges`, switched to the
  existing identity-aware workspace fixture.
- Re-walk: the window opened keyed `ws_e72b1865b5dccbd5` with the file open.

### Open finding — transient `500 changes_unavailable` (not reproduced)

Shortly after the lab daemon restart, two patch batches and one summary call failed with the
engine's "git read failed" error. Diff bodies stayed empty behind per-file retry controls. A fresh
load rendered all diffs, and 50 concurrent requests all returned 200. The daemon logs only the status,
not the git error, so there is no root cause. Next step: log the underlying git error in
`changeError`'s `ErrUnavailable` branch, then re-run.

### Observations

- After "Compare against <ref>" with a ref that does not exist, the base popover stays open. While
  that base is selected, each refresh retries and the console logs the `409`.
- An in-root credential file gets the native sheet title "Open file outside this workspace?"; only
  the detail line ("It may contain credentials.") differs. That copy predates this spec (#1167).
- The invalid-regex message is the daemon's raw text (S9 residual).
- `compozy workspace add` writes `.compozy/workspace.toml` into the project, so it shows as an
  untracked change (251 files in the 250-file project).

## Review remediation (frontend and desktop)

Branch `feat/file-explorer-fix-frontend`, based on `c24a0f887`. It fixes the Opus implementation
review findings O-1 to O-10 (`.compozy/tasks/file-explorer/qa/impl-review-opus.md`). Backend
findings are fixed on a separate branch.

| Finding | Fix | Covering test |
| --- | --- | --- |
| O-1 Open in menu | The configured editor is listed only when `editor_command` is set. Catalog has all 13 spec editors. Menu: icons, separators, file manager (Finder / Show in folder / Show in Explorer), Copy path, Refresh list / Editor settings… footer, "Set an editor in Settings" when no editor exists. Docs and `APP-open-in-targets` were rewritten back to this spec behavior. | `file-opener.test.ts` (configured gating, catalog order, platform labels, new argv cases); `open-in-button.test.tsx`; desktop E2E-008 |
| O-2 Tree row menu | `FileTree` renders `rowMenu` through `DropdownMenu`, anchored at the pointer. Rows offer Copy path, plus the Open in targets in the desktop app; directories open without a line. The root header has **Open folder in…**. | `packages/ui` `file-tree.test.tsx`; `open-in-button.test.tsx` (`FilesTreeRowMenu`) |
| O-3 Silent expand failure | `DiffViewerItem.expandable`; on `too_large_to_expand` or sensitive contents the expand control goes and the header says "File too large to expand" or "Sensitive file · context stays hidden". | `diff-viewer.test.tsx`; `use-session-changes.test.tsx` |
| O-4 Search ↑/↓ focus | The find bar focuses only on an explicit Cmd/Ctrl+F, not when seeded from Search. | `file-viewer-pane.test.tsx` ("keep stepping Search results") |
| O-5 Keyboard tests | `[`/`]`, last-file boundary, Alt+S (persisted, ignored in inputs and narrow windows), result-list ↑/↓. | `use-session-changes.test.tsx`; `file-viewer-pane.test.tsx` |
| O-6 Vanished editor | The toast names the editor ("Cursor is no longer available."), and the target list is re-detected, from both the button and Cmd-click. | `file-opener.test.ts`; `open-in-button.test.tsx` |
| O-7 Credential confirmation | In-root credential files get "Open a file that may contain credentials?"; the outside-path text is unchanged. | `file-opener.test.ts` |
| O-8 Target line, blank paint | CodeViewer target line uses `--color-brand` through Pierre's selection overrides. Light row contrast went from 1.03:1 (ΔE 1.0) to 1.26:1 (ΔE 15.3); dark went from 1.05:1 to 1.34:1 (ΔE 23.3). A plain numbered-text paint shows until the highlight renders. | `code-viewer.test.tsx`; Storybook captures in both themes |
| O-9 Link tooltip | The tooltip reads "⌘-click opens in <last-used target>". | `message-markdown.test.tsx` |
| O-10 Missing base | A persisted `base_not_found` base is cleared back to the default, with a notice. | `use-session-changes.test.tsx` |

Operator assumptions:

- **US-010.AC-2:** focusing the session's in-app OS window refreshes the Changes summary
  (`useShellFocusedSessionId`). Covered by `use-session-changes.test.tsx` and
  `os-interaction-hooks.test.tsx`.
- **US-004.AC-1:** the image preview caption shows "W × H · size". Covered by `code-viewer.test.tsx`.
- **US-001.AC-4:** implemented. The listing already carries `contains_changes`. An unloaded changed
  folder feeds Pierre one placeholder descendant status, so it shows the tree's own dot. Covered by
  `file-tree.test.tsx` and `file-tree-paths.test.ts`; Storybook captures in both themes.

Verification (every `COMPOZY_*` variable unset for E2E):

```sh
cd web && bunx playwright test e2e/__tests__/files-explorer.spec.ts e2e/__tests__/session-changes.spec.ts  # 10 passed
bun run --cwd desktop build:e2e-update                                                                     # ok
cd desktop && bun x playwright test --config playwright.config.ts --grep 'E2E-008|E2E-001 \(#1167\)'      # 3 passed
bunx turbo run typecheck --filter=./web --filter=./packages/ui --filter=./desktop                           # 6/6 ok
```

Real-browser checks (Storybook, light and dark): the tree row menu opens at the pointer; the unloaded
folder dot matches the loaded one; the target line is visibly tinted. Captures are in
`/tmp/fe-fix-shots/`.

Still unverified: real editor launches, native confirmation sheets, and Finder reveal on a packaged
build with real apps installed. `APP-open-in-targets` stays `partial`.

## Commands

```sh
python3 .agents/skills/eng/eng-qa-bootstrap/scripts/bootstrap-qa-env.py --scenario file-explorer-integration \
  --repo-root . --profile targeted --required-surface web --required-surface api --required-surface cli
go build -o /tmp/s10/compozy ./cmd/compozy && (cd web && bun run build)
go test ./internal/api/core -run 'TestProjectFileContentReveal|TestProjectChanges|TestSearchSessionFiles' -count=1   # ok
go test -ldflags=-w ./internal/daemon -run '^TestNativeSessionChanges$' -count=1                                     # ok (red without the fix)
GOFLAGS=-ldflags=-w go test -tags integration ./internal/daemon -run '^TestDaemonSearchSessionFilesWorktreeBoundIT005$' -count=1  # ok 16.5s
golangci-lint run --allow-parallel-runners --concurrency 2 ./internal/api/core ./internal/daemon                    # 0 issues (pinned 2.15.0)
```

The lab was torn down with the manifest's `TEARDOWN_COMMAND`; see the lab's
`qa-artifacts/qa/teardown.json`.
