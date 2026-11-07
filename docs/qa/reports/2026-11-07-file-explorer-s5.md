# File explorer S5 frontend evidence

Scope: S5 of `.compozy/tasks/file-explorer/_slices.md` on `feat/file-explorer-s5` (base
`feat/file-explorer` at `818ba3d5d`: S1 file routes, S2 changes routes, S3 primitives, S4 Review
panel, S8 search routes). Frontend only: no wire, schema, config, hook, native tool, or CLI change.
The owning cross-surface audit is the spec's `_impact-entry.md`; S10 owns docs, the official skill,
and QA scenarios. Affected scenarios: the new `ET-session-review-changes` (scopes, base, large
mode, refresh) and `ET-session-review-files` (a changed-file row falls back to Files).

## Implementation

- Data (`web/src/systems/session`): `adapters/session-changes-api.ts` wraps the four S2
  operations with a typed `SessionChangesApiError` (daemon code, `AbortSignal`).
  `lib/session-changes-query.ts` owns keys and options (summary keeps the last good data on a
  failed refresh; window focus refreshes). `lib/session-changes.ts` maps a summary to diff items
  (version = revision + fingerprint, so a refresh reloads exactly the files that moved), tree
  decorations, the large-diff policy (more than 200 files or 20,000 changed lines), and the
  first changed line for Open in viewer. `lib/session-changes-loading.ts` holds the patch batcher
  (same-tick loads share requests, capped at 20 paths and 8 KiB per batch), the debounced refresh
  (750 ms quiet, 2 s ceiling), and the stale-fence state machine.
- `hooks/use-session-changes.ts`: summary read, batched patches fenced by the summary revision and
  each file's fingerprint (`409 changes_stale` or a fingerprint mismatch → one summary refetch,
  then the stale items reload; a second stale read stops at "Couldn't refresh · Retry"), contents
  for context expansion, and per-file notes (too large, sensitive, no content).
- `hooks/use-session-changes-live-refresh.ts`: watches what the session's live stream already
  writes (transcript cache: settled tool calls; session read: an ended turn) and debounces into
  one summary refetch. It opens no stream of its own.
- `hooks/use-session-changes-view.ts` + `use-session-change-selection.ts`: selection (tree,
  `[`/`]`, scroll), the one-file stepper, collapse/reveal choices kept across refreshes,
  scope/base/mode preferences, narrow windows (< 900 px session window: unified only, tree behind
  its toggle), and the focus request from a changed-file row.
- Components (`components/session-changes/`): header row (scope menu, totals with lower-bound
  tooltip, `head → base ▾` selector with server-filtered branches, default marker, and any
  git-resolvable ref such as `main~1`; PR link; Refresh, split/unified, collapse all, files tree),
  `DiffViewer` list with a per-file header (status letter, path or `old → new`, Generated pill,
  lock, `+N −M`, collapse, Open in viewer, Copy path, and the binary / Show contents / too large /
  retry notes), the changed-files `FileTree`, and the loading, empty, not-a-repo, base-not-found,
  error, refresh-failed, large-banner, and truncated states.
- Review panel store: tabs `changes` and `files`; `diffMode`, `scope`, `base`, and `changesTree`
  persist under `compozy:session:review-panel:v1` (stored `tab` is now honored); the focus request
  is transient. `requestSessionReviewChanges(sessionId, { path?, scope? })` is the entry point.
- Entry points: the PR chip menu's **Review changes** (Branch scope; absent in user scope), and a
  turn's "Changed files" rows, which are now buttons that open Changes at the file, or Files when
  the summary doesn't list it (`useFilePathLinks().openChange`, reusing S4's root resolution).

## Test ownership and mapping

| ID | Suite / case |
| --- | --- |
| UT-053 | `web/src/systems/session/hooks/__tests__/use-session-changes.test.tsx`: changed-file routing › Should focus a listed file in Changes and open Files for an unlisted one |
| UT-056 | `web/src/systems/session/lib/__tests__/session-changes.test.ts`: isLargeDiff › Should classify totals … |
| UT-057 | `use-session-changes.test.tsx`: useSessionChangesLiveRefresh › Should refresh the summary once for three tool calls completing within one second |
| UT-058 | `use-session-changes.test.tsx`: useSessionChanges › Should refetch once on a stale patch and stop at a retry state on the second |
| E2E-004 | `web/e2e/__tests__/session-changes.spec.ts`: E2E-004 (fixture `internal/testutil/acpmock/testdata/browser_session_changes_fixture.json`) |
| E2E-005 | same spec: E2E-005 |
| E2E-006 | same spec: E2E-006 |
| E2E-007 | same spec: E2E-007 |

Two new suites: the Changes hooks and lib had no existing owner. The existing
`use-session-topbar-slot.test.tsx` gained one case for **Review changes** (US-007.AC-8), and its
two menu-inventory assertions now list the new `review-changes` item (the menu's real contents,
not a weakened check).

## Verification

From the repository root unless noted; all passed on the final head:

```sh
bunx turbo run typecheck --filter=compozy-web   # first run; later iterations: cd web && bun run typecheck:raw
cd web && bunx vitest run src/systems/session/lib/__tests__/session-changes.test.ts \
  src/systems/session/hooks/__tests__/use-session-changes.test.tsx                  # 2 files, 7 tests
cd web && bunx vitest run src/systems/session/hooks/__tests__/use-session-topbar-slot.test.tsx \
  src/components/assistant-ui/__tests__/session-thread.test.tsx \
  src/systems/session/components/__tests__/message-markdown.test.tsx \
  src/systems/session/lib/__tests__/file-path-links.test.ts                         # touched suites
bunx oxlint --deny-warnings <changed files>; bunx oxfmt --check <changed files>
cd web && bunx react-doctor --no-score --no-supply-chain --no-telemetry --scope changed \
  --base 818ba3d5d --include-untracked                                              # no issues
cd web && bunx playwright test e2e/__tests__/session-changes.spec.ts                # 4 passed
```

E2E-004–E2E-007 passed in one Playwright run (1 worker, 1.6 min including a throwaway visual
spec). The run must unset inherited `COMPOZY_*` variables when launched from a managed session
(see residuals).

Real-browser check (Chromium through the same harness, 1600x1000, light and dark via
`compozy.theme`): a branch with a committed edit, an uncommitted edit, untracked text, a sensitive
`.env`, a generated `bun.lock`, and a binary file. Checked: header row (Branch ▾, `+5+ −1+`
lower-bound totals because of the binary untracked file, `feat/review → main ▾`, toolbar), the
sensitive gate with Show contents, the collapsed Generated file, "Binary file changed", status
letters, unified and split diffs with "27 unmodified lines" expansion, the changed-files tree
with counts and status, the base selector (filter, `default` marker), and the scope menu. Captures
were kept outside the repo; no PR evidence is part of S5.

## Deviations and residuals

- Context expansion over 1 MiB (US-007.EC-5): Pierre owns the expand control, so it stays visible;
  the daemon's `too_large_to_expand` makes the expansion fail quietly instead of hiding the
  control with a tooltip. Needs an S3 `DiffViewer` per-item "expandable" flag.
- "Scrolling the diffs updates the highlighted row" uses the viewer's rendered items (visible plus
  overscan): the selection moves only when the selected file leaves that window.
- The narrow threshold measures the session window split (900 px), not the Review panel.
- The Review panel still opens on Files by default (S4's default); Changes opens from its tab,
  the PR chip, or a changed-file row.
- E2E runs from a Compozy-managed session must drop inherited `COMPOZY_*` variables
  (`COMPOZY_INTERNAL_RESTART_OPERATION_ID` makes the harness daemon exit before readiness).
