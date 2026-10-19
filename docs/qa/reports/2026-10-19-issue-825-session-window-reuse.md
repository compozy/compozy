# 2026-10-19 — New sessions reuse the Session window (#825)

Targeted lab: the web Playwright harness (`web/e2e/fixtures/runtime.ts`) with an isolated temp
`COMPOZY_HOME`, operator home, HTTP port and socket per test, a prebuilt daemon from branch
`feat/issue-825-new-session-reuse-window` (`COMPOZY_TEST_DAEMON_BIN`), the acpmock
`os-shell-multi-session-agent`, `new_window_policy = "floating"`, headless Chromium 1280×720.
Scenario: `ET-web-new-session-reuses-session-window`.

## Walk

| Step | Action | Observation | Verdict |
| ---- | ------ | ----------- | ------- |
| 1 | Session window focused with an unsent draft; Session > New session | Same window id shows the new session and stays focused; one Session window | pass |
| 1b | Pick the previous session in the window's sidebar | Same window; the unsent draft is still in the composer | pass |
| 2 | Tasks focused; Session > New session | Session window comes forward with the new session; no second window | pass |
| 3 | Session window minimized; Session > New session | Window restored with the new session; daemon reports `minimized: false` | pass |
| 4 | Sessions A and B plus an empty-state window, Tasks focused; create | Empty-state window receives it; A and B unchanged; three Session windows | pass |
| 5 | Session window only on Desktop 1, Desktop 2 active; create | New window on Desktop 2; Desktop 1 window untouched | pass |
| 5b | Only Session window pinned; create | New window opens; pinned window keeps its session | pass |
| 6 | No Session window; create | New window opens | pass |
| 7 | Fork / Continue with New window | Opens beside the source (`session-derive.spec.ts` E2E-002/001/004) | pass |
| 8 | Minimized Session window on an otherwise empty desktop; send from the empty-desktop composer | Window restored with the new session and its first prompt | pass |

Evidence: <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case1-focused-before.png>, <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case1-focused-after.png>, <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case1b-draft-kept.png>,
<https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case2-unfocused-before.png>, <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case2-unfocused-after.png>, <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case2b-minimized-before.png>,
<https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case2b-minimized-after.png>, <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case3-several-before.png>, <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case3-several-after.png>,
<https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case5-other-desktop-before.png>, <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case5-other-desktop-after.png>, <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case6-pinned-before.png>,
<https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case6-pinned-after.png>, <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case4-none-before.png>, <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case4-none-after.png>,
<https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case7-desk-composer-before.png>, <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case7-desk-composer-after.png>.

Teardown: each harness runtime removed its temp home on exit.

## Session Debriefs

- Not walked in the lab: the palette prompt fallback (needs `cmd_palette.fallback_targets`); it shares the
  empty-desktop composer's `created` jump, which step 8 exercised, and `use-os-palette-root.test.tsx`
  pins its payload. Cross-workspace creation from Global scope is covered by the coordinator unit test
  for the workspace-switch barrier, not by a lab walk.
