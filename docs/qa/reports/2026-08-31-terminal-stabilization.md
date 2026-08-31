# 2026-08-31 — Integrated terminal stabilization + window-native UX walk

Scope: `docs/prompts/20260831-1333_integrated-terminal-stabilization-ux-rework.md`
(branch `integrated-terminal`, commits `aa25aa11a` + the follow-up batch).
Walked by automation against real daemons: an isolated CLI lab
(`COMPOZY_HOME`/`HOME`/`XDG_*` under `/tmp/compozy-walk`, port 21873, dedicated
pids, torn down at the end) plus the full Playwright browser journeys
(`web/e2e/__tests__/terminal.spec.ts` — 12/12, `terminal-agent.spec.ts` — 7/7)
each booting its own daemon from the branch binary.

## Verdicts

| Scenario | Verdict | Primary evidence |
| --- | --- | --- |
| ET-terminal-browser-lifecycle | pass | E2E-002 (dock → working terminal, deck tabs, reload, close keeps sessions, reopen adopts) + E2E-018 |
| ET-terminal-window-native-flow | pass | E2E-002 (head New → deck tab), E2E-007/E2E-018 (head Journal toggle + back), E2E-009 (cap dialog from head New), unit UT-122/createIntent (dock-menu create intent) |
| ET-terminal-shell-config-fidelity | pass | CLI lab: fish `fish_config theme choose rose-pine` → `THEME-OK`; user function `walkfn` ran; `WALKPROBE confdir=$HOME/.config/fish`; journal rows `detected: marker, exit: 0` for fish and zsh; zsh startup clean |
| ET-terminal-cli-public-contract | pass | E2E-016 (kill exited = idempotent success; signal exited = `terminal_exited`), E2E-001/011 transcripts |
| ET-terminal-stream-resilience | pass | protocol-client suite (43 cases incl. stop-on-exit, vote flush, fatal codes), E2E-002 reload reattach, E2E-011 detach/keep-running |
| ET-terminal-profile-segmentation | pass | E2E-020 (per-profile desktops, fresh terminal per profile, aggregate journal owners, badge re-scope) |
| ET-terminal-journal-recording | pass | E2E-007 (head Journal toggle, filters, replay, estimated rows) |
| SITE-terminal-docs-truth | pass | `packages/site/content/docs/terminal/*` audited — CLI/agent-focused, no claims about the removed tab strip or launcher; `_dx.md` kill transcript updated to the idempotent contract |

## Bugs found by the walk (fixed in-branch)

1. **zsh markers never worked**: the marker script's `local status=$?` shadows
   zsh's read-only `status` parameter — the precmd hook died on every prompt
   ("read-only variable: status") and no `F;exit=` marker was ever emitted in
   zsh. Fixed in `internal/terminal/pty/shell_integration.go`; pinned by live
   test UT-123.
2. **Resolver could duplicate instead of adopting**: the id-less route decided
   on a cached catalog from before the window opened; a terminal created after
   page load was missed and a duplicate was opened. Fixed by gating the
   resolver on `catalog.isFetchedAfterMount`; pinned in the window suite.

## Follow-up findings (out of scope, not fixed here)

- fish startup waits ~10s for a Primary Device Attribute response when no
  viewer is attached to answer terminal queries (headless/detached opens);
  with a live web viewer xterm answers and there is no wait.
- `web/e2e/fixtures/os-navigation.ts#ensureAppWindow` dock-clicks when its
  visibility probe races a reconciled window, which minimizes a freshly
  focused window (dock toggle semantics). Deep-link flows now wait for the
  surface directly; the fixture race remains for other suites to trip on.
