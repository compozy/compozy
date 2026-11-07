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
