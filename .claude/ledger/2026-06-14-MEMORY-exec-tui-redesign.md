# Memory Ledger — Execution TUI Redesign

- **Goal (success criteria):** Redesign the execution (ACP COCKPIT) TUI in `internal/core/run/ui` to match the wizard's polish. (1) Single-line job rows: marker+icon+2-digit number+real title + right-aligned elapsed/duration; drop FILES/ISSUES. (2) Runtime line adds reasoning + input/output token split `Codex · gpt-5.5 · xhigh · ↑8.1k ↓4.2k`. (3) Header adds aggregate tokens `RUN 2/3 · 48.6k tok`. (4) Polish: sidebar pane title, tab-bar glyphs, spacing. Then loop `/cy-impl-peer-review` until SHIP. `make verify` must pass 100%.

- **Constraints/Assumptions:**
  - All data already on UI model (`uiJob.taskTitle/.reasoningEffort/.tokenUsage`, `uiModel.aggregateUsage`). Rendering-only change.
  - ANSI-safety: truncate plain text, then style, then join. Never `truncateString` a styled string.
  - Reuse `charmtheme` tokens + existing `render*` helpers. No new deps. No panic/log.Fatal.
  - Skills: golang-pro, bubbletea, tui-design, tui-glamorous, testing-anti-patterns, cy-final-verify. `make lint` zero tolerance.

- **Key decisions:** User-confirmed via AskUserQuestion: sidebar=Title+compact time; runtime=input/output split; header=add aggregate tokens.

- **State:** COMPLETE.
- **Done:** All 4 changes + tests (198 pass run/ui). Fixed pre-existing flaky cli os.Stdout-swap race (removed global swap in executeCommandCapturingProcessIO). Peer review (Opus subagent; compozy exec blocked by daemon mismatch + 2 active user runs, not force-stopped) → verdict SHIP round 1 (0 blockers, 4 risks, 5 nits). Incorporated all actionable risks/nits (N-004 deferred). `make verify` EXIT=0 (0 races, 0 fails, 0 lint). Artifacts in .peer-reviews/20260614T172657Z/.
- **Now:** Done. Awaiting any user follow-up (commit/PR not done — needs explicit ask).
- **Next:** (optional) commit when user requests; force-stop daemon only with permission.
- **Open questions:** none.

- **Working set:**
  - `internal/core/run/ui/sidebar.go` — renderSidebarItem(+index), delete sidebarMeta, add formatTokens, sidebar title
  - `internal/core/run/ui/types.go` — sidebarRowCacheKey fields, sidebarRowLines const
  - `internal/core/run/ui/update.go` — refreshSidebarContent: pass index, scroll math *3→*1
  - `internal/core/run/ui/timeline.go` — timelineRuntimeMeta reasoning+tokens, styled meta row
  - `internal/core/run/ui/view.go` — headerStatusText aggregate tokens
  - `internal/core/run/ui/layout.go` — sidebar viewport height reserve title row
  - `internal/core/run/ui/multi_remote.go` — renderTabs glyphs
  - Tests: `view_test.go` (L154 renderSidebarItem, L294 timelineRuntimeMeta, L438/443 headerStatusText), `update_test.go` (scroll)
