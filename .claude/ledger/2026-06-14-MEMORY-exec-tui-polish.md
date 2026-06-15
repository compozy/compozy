# Memory Ledger — Execution TUI Production-Grade Polish (wizard parity)

- **Goal (success criteria):** Implement the accepted plan
  `~/.claude/plans/veja-o-handoff-n-s-delightful-coral.md` for `internal/core/run/ui` (ACP Cockpit):
  (1) terminal-background passthrough package-wide (drop forced bg, mirror wizard); (2) fix multi-run
  spinner freeze on tab switch; (3) tighten header top margin; (4) tighten footer; (5) wizard rhythm
  (HR above footer, charmtheme.Keycap, spacing) keeping per-pane borders; (6) sidebar job rows as padded
  cards (L1: marker+icon+NN+title, L3 muted: task type · elapsed/tokens); (7) official task numbering
  (task_NN.md via tasks.ExtractTaskNumber, fallback index+1). Then loop cy-impl-peer-review to SHIP.
  `make verify` 100%, zero lint.

- **Constraints/Assumptions:** no destructive git; no workarounds (root-cause); bubbletea/lipgloss v2
  (`charm.land/*/v2`); keep accent _chips_ (active tab, [x], selected button) — only drop frame/surface fills.
  Skills active: golang-pro, bubbletea, tui-design, tui-glamorous, systematic-debugging, no-workarounds,
  testing-anti-patterns, cy-final-verify.

- **Key decisions (user-confirmed):** bg removal = whole run/ui package; sidebar = padded cards;
  chrome = wizard rhythm + keep pane borders (no outer frame).

- **State:** IMPLEMENTED + VERIFIED (make verify EXIT=0). Peer-review pending.
- **Done:** All 7 defects implemented. styles.go bg-free API (dropped bg params + reapply machinery);
  view.go chrome (header 2-row no leading blank, footer 1-row, HR above footer, charmtheme.Keycap);
  sidebar.go 2-line cards (title + muted meta: type·elapsed·↑in↓out) + official numbering;
  multi_remote.go parent-owned spinner (ensureSpinnerTick keyed on hasAnyActiveTab; restart on tab
  switch + child job-start) + tabs/dialog bg-free; numbering plumbed model.Job→runshared.Job→
  JobQueuedPayload→snapshot/contract→ui (tasks.ExtractTaskNumber, fallback index+1); sdk/extension.Job
  aligned. timeline/summary/validation_form/review_watch migrated by 4 parallel subagents. types.go
  chromeHeight=5, sidebarRowLines=3. Tests updated (assertNoForcedBackground; 2-line card assertions;
  golden +1 row; 3 new tests: 2 spinner-freeze + 1 official-number). `make verify` EXIT=0, `make lint`
  0 issues, ui pkg 201 tests pass.
- **Now:** Run cy-impl-peer-review until SHIP (note: compozy exec may be blocked by daemon mismatch +
  active user runs — do NOT daemon stop --force without permission).
- **Next:** Address any peer-review blockers; final report. Commit only if user asks.
- **Open questions:** none.

- **Working set:** internal/core/run/ui/{styles,view,sidebar,timeline,summary,multi_remote,update,model,
  layout,types}.go (+ \_test.go); internal/core/plan/prepare.go; internal/core/model (Job);
  internal/core/run/internal/runshared/config.go; pkg/compozy/events/kinds/job.go;
  internal/daemon/run_manager.go.
