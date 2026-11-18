---
id: ET-session-review-changes
area: ET
title: Review a session's branch and uncommitted changes in the Changes tab
persona: Dora
journey: J-14
expected: The Review panel shows only Changes (no tab strip): the session root's diff against its base with scope, head → base, totals and a changed-files tree; sensitive sides stay gated, generated files collapse, split/unified persists; Uncommitted drops committed files; any git-resolvable base works and a missing one shows "Base <ref> not found" with Choose base; agent edits appear without a manual refresh and external edits appear on Refresh; a turn's changed-file row focuses that file; diffs over 200 files or 20,000 lines show one file at a time; a file too large to expand says so instead of failing silently; a saved missing base falls back to the default with a notice; focusing the session window from another in-app window refreshes the summary.
entry_points: web session Review panel (Changes toggle in the session head); PR chip menu Review changes; a turn's Changed files rows; compozy session changes
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: lab compozy-file-explorer-integration-20261107-073905-206925 (qa-artifacts/qa/journey-log.jsonl, screenshots/C01–C10, notes/cli-*.txt|json)
last_report: docs/qa/reports/2026-11-07-file-explorer-integration.md
overlaps: ET-transcript-paths-files-app; APP-open-in-targets
---

Introduced by the file-explorer spec (US-005.AC-6, US-007–US-010). Walk it on a real daemon with a
git project on a feature branch that has a committed edit, uncommitted edits (including a sensitive
`.env` and a generated lock file), an untracked file, and an agent that edits files.

Walk:

1. Open the Review panel with **Changes** in the session head. It has no tab strip. The header shows the scope (**Branch**),
   `head → base` (for example `feat/review → main`), the totals and the toolbar. The tree and the
   diff list hold the same files.
2. Select `.env` in the tree: its diff is gated behind **Show contents**. The generated lock file
   shows a **Generated** pill, collapsed.
3. Switch to split, reload: the mode persists, along with the scope and base.
4. Switch the scope to **Uncommitted**: committed-only files leave the list. Switch back to Branch.
5. Open the base selector, filter, and choose a branch or a typed ref (`feat/review~1`) through
   "Compare against …": the summary follows. Type a missing ref: the panel shows "Base <ref> not
   found" with **Choose base**, which recovers.
6. Ask the agent to create a file. Its header appears without a manual refresh, within about 2 s of
   the tool call.
7. Edit a file outside the agent and choose **Refresh**: the new line appears.
8. In the turn's "Edited N files" row, choose **Review changes in <file>**: Changes selects that
   file and scrolls it into view.
9. In a project with more than 200 changed files, the panel shows "This diff is large, showing one
   file at a time" with `1 of N`. The next-file stepper moves to `2 of N` while the tree lists every
   file.
10. `compozy session changes <id>` prints the same summary. `--scope uncommitted --patch <path>`
    adds patches; a sensitive patch carries only `sensitive: true` until `--reveal`; a User-scope
    session exits 1 with `root_unavailable`, and a missing base with `base_not_found`.
11. With a forge-linked PR, the PR chip menu's **Review changes** opens Changes on Branch scope.
12. Review remediation (2026-11-07): expand context on a file over 1 MiB. The expand control goes
    away and the header says **File too large to expand**; nothing fails silently. Press `]` and `[`
    to step files (Next is disabled on the last file) and Alt+S to flip split/unified (ignored while
    typing in the tree filter or in a narrow window).
13. Save a base, then delete that branch and reopen the session: Changes falls back to the default
    base with "Base <ref> no longer exists. Showing changes against the default base." and the
    browser stops re-requesting the missing base on focus.
14. Edit a file outside the agent, switch to another CompozyOS window, then back to the session
    window: the summary refreshes without pressing Refresh.
15. Deep, long paths (#1480): on a branch with ~60 changed files six to eight levels deep that share
    a long prefix (`packages/<pkg>/proto/review/<long-dir>/attachments/<uuid>.png`), check the panel
    at the default width and in a narrow window. Tree names truncate behind one clean "…" that
    never overlaps the text, and the extension stays. A single-child directory chain shows as one
    `proto / review` row, and the indent leaves room for names. Every file row shows `+N −M` and
    its status in a trailing column. Hovering a row shows its full path. Diff headers drop the
    start of a long path (`…customer-detail/admin-billing-fixtures.ts`), so sibling rows differ.

Owning automated coverage: `@compozy/ui` `file-tree.test.tsx` and the FileTree "Deep changed files"
stories (step 15); web `session-changes.test.ts` (UT-056), `use-session-changes.test.tsx`
(UT-053, UT-057, UT-058), `use-session-topbar-slot.test.tsx` (Review changes), and
`web/e2e/__tests__/session-changes.spec.ts` E2E-004–E2E-007; engine `TestEngine` and
`TestEngineLimitsAndRaces`; CLI and hosted MCP parity in `TestDaemonSearchSessionFilesWorktreeBoundIT005`.

## Walk 2026-11-07

Same lab and build as `ET-transcript-paths-files-app` (integration `410a3566e`, fix `c5d1d54a6`). A second
project held 250 untracked files for large mode.

| Step | Result | Evidence |
| --- | --- | --- |
| 1 Branch summary | pass | `Branch`, `feat/review → main`, `+8−3`, 6 files (`C01-branch-summary.png`) |
| 2 Sensitive and generated | pass | `.env` gated (`C02-sensitive-env.png`); `bun.lock` header "Generated +1−1" |
| 3 Split persists | pass | `split` after reload (`C03-split.png`) |
| 4 Uncommitted | pass | 5 files, `src/b.go` dropped (`C04-uncommitted.png`) |
| 5 Base selector and missing base | pass | `feat/review~1` applied (`C05-base-custom.png`); "Base nope-ref not found" + Choose base recovered to `main` (`C06-base-not-found-closed.png`) |
| 6 Live refresh | pass | `e.go` header 70 ms after the reply rendered (`C07-live-refresh.png`) |
| 7 Refresh after an external edit | pass | `C08-refresh.png` |
| 8 Changed-file row | pass | `src/b.go` selected (`C09-changed-row-focus.png`) |
| 9 Large mode | pass | Banner, `1 of 251` → `2 of 251`, one header, tree 251 (`C10-large-mode.png`; the 251st file is the project's `.compozy/workspace.toml`) |
| 10 CLI | pass | `notes/cli-changes-human.txt`, `notes/cli-changes-uncommitted.json`, `notes/cli-api-edges.txt` |
| 11 PR chip | not walked | The lab has no forge remote; `use-session-topbar-slot.test.tsx` covers the menu entry |

Open finding (not reproduced): in the first minutes after a lab daemon restart, two
`GET …/changes/patches` calls and one summary call returned `500 changes_unavailable`, the engine's
"git read failed" case. The diff list then showed headers with a per-file retry control and no
bodies (`C02-sensitive-env.png`, `C03-split.png`). A fresh load rendered every diff
(`C11-diff-bodies.png`), and 50 concurrent patch and summary requests all returned 200. The daemon
logs the status but not the underlying git error, so the cause is unknown. Logging that error is the
next diagnostic step.

Observations, not defects: after "Compare against <ref>" with a ref that does not exist, the base
popover stays open; with a valid ref it closes. While the missing base is selected, each focus or
live refresh retries the summary, and the browser console logs the `409 base_not_found` response.

## Walk 2026-11-10 (#1480)

Lab `compozy-issue-1480-file-tree-20261111-001512-265025` (targeted web/api, `acpmock` agent), a
`feat/billing-review` branch with 58 changed files under
`packages/frontend/proto/review/admin-billing-*/`, fix commit `d12f6acbb`.

| Step | Result | Evidence |
| --- | --- | --- |
| 15 Deep paths, default width (1600 px) | pass | Before: `stomer-detail`, `attaments`, `3c:png`, no counts, every header `packages/frontend/proto/review/admin-b…` (`01-changes-before-default.png`); after: `proto / review`, `admi…ustomer-detail`, `3c9b…png +1 −0 A`, headers `…tomer-detail/admin-billing-fixtures.ts` (`02-changes-after-default.png`) |
| 15 Deep paths, narrow (1100 px) | pass | `03-changes-before-narrow.png`, `04-changes-after-narrow.png`; headers keep `…res.ts`, `…3a.png` |

## Backend consistency and diagnostics regression checks

- Edit a requested file between the summary and patch reads from the CLI or native tool. The
  combined consumer must refresh summary and patches once. If another edit makes the retry
  inconsistent, return `changes_stale`; the CLI exits non-zero and includes the domain code.
- Cause a Git diff failure or timeout. Every Changes 5xx from HTTP/UDS (including requests made
  by the CLI) and the native tool must record one warning containing the Git cause and subcommand.
  The absolute root is redacted and the public message remains fixed.
- The owning regression suites are `TestEngine`, `TestProjectChangesDiagnostics`,
  `TestSessionChanges`, and `TestNativeSessionChanges`. They cover deterministic failures and
  concurrent edits. The original transient restart incident still has no proven root cause.

- Request a changed path omitted from a truncated summary page. A stable patch must be returned;
  summarized paths still require exact fingerprint equality.
- Configure a missing external `diff.orderFile` in an isolated fixture and request Changes.
  The warning must classify the Git subcommand, exit status and timeout without retaining that
  external path. Path-bearing diagnostic lines are omitted, including quoted, Windows and home paths.

- Read a truncated/partial report through `compozy session changes`. Human output must state the
  summary file limit and that totals are a lower bound. Requested oversized or unavailable patches
  must be named explicitly; JSON and JSONL keep the existing structured flags.

## Backend batch and overlap checks

Request 20 changed files together. The batch uses one rendered Git diff, bounded per-file patch
buffers and matching before/after fingerprints. Untracked additions are formatted with go-gitdiff
from confined reads; sensitive items remain gated. Concurrent identical summaries share only the
in-flight work; a later request still computes a fresh pair of snapshots.

Replace a tracked directory with a regular file and request its patch alongside another modified
file. Both additions must remain available. Repeat with a tracked file replaced by a directory and
with a gitlink whose commit is absent from the parent object database. Unsupported current kinds
return an empty per-item patch/fingerprint; the unrelated file must still return its patch.

For concurrent summaries, cancel the first request while Git is active. Another active request
must complete by retrying canceled shared work. Cancel all requests and verify Git work stops.

Through both CLI and the native tool, request one valid patch and one current directory. The
report must retain both items and the valid patch instead of reporting changes_stale. Items with
content still require summary fingerprint equality; unavailable items have no content to fence.

Security errors are not unavailable patch items. Request a symlink into protected Git metadata,
a symlink outside the approved root, and an invalid relative path through patches HTTP/UDS,
CLI and the native tool. Preserve path_denied, path_outside_root and invalid_path respectively;
return no partial patch batch and disclose no absolute host path. Verify these outcomes both
while obtaining fingerprints and if denial appears during sensitivity or untracked-content reads.

## 2026-11-13 qa-impact (grouped file menu)

In the desktop app, each changed (not deleted) file header gains a **More actions** menu after Copy
path: the shared file menu led by **Open in Files**. Its walk lives in `APP-open-in-targets` step 5;
browser tabs show no such menu, so this scenario's browser walk is unchanged.
