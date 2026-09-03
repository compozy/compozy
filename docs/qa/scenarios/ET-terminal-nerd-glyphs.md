---
id: ET-terminal-nerd-glyphs
area: ET
title: Terminal renders Nerd Font glyphs instead of tofu
persona: Marina
journey: J-operate-integrated-terminal
expected: A shell prompt that uses Nerd Font icons (powerline separators, git branch, devicons — Private Use Area codepoints) renders real glyphs in the web terminal, not empty boxes; glyphs appear without any font installed on the viewer's machine, in both the WebGL and DOM renderers, and a face that finishes downloading after attach repaints the grid instead of leaving stale tofu.
entry_points: Web dock Terminal app; /terminal; /terminal/{terminal_id}
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: Verified by Pedro on the live dev stack (worktree daemon + Vite dev, 2026-09-01) against a real Nerd Font prompt that previously rendered tofu; wiring owned by packages/ui/src/components/terminal/__tests__/terminal-view.test.tsx (atlas clear on late font load) and __tests__/terminal-fonts.test.ts (stack loading, budget, residency).
last_report:
overlaps: ET-terminal-browser-lifecycle
---

Flagged 2026-09-01 by the Nerd Font glyph fix: the bundled JetBrains Mono ships
no Private Use Area icons and browsers resolve no system fallback for PUA
codepoints, so prompts drew tofu. The mono stack now carries a bundled
"Symbols Nerd Font Mono" face (unicode-range-scoped) and terminal attach loads
the stack explicitly, clearing the glyph atlas when a face lands late.

Walk:

1. Open the Terminal app in a project whose shell prompt uses Nerd Font icons (starship, powerlevel10k, or `printf '   \n'`).
2. Confirm the icons render as real glyphs — no hollow boxes — with no Nerd Font installed on the viewing machine.
3. Reload with a cold cache and confirm the grid repaints the icons once the symbols face finishes downloading.
4. Confirm regular text still renders in JetBrains Mono (the symbols face never serves Latin codepoints).
