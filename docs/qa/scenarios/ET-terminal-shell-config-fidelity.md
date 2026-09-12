---
id: ET-terminal-shell-config-fidelity
area: ET
title: A terminal shell keeps the user's own configuration
persona: Marina
journey: J-operate-integrated-terminal
expected: fish opens with the user's themes, functions, completions, and universal variables intact while journal markers still record commands; zsh preserves native ZDOTDIR semantics and startup ordering while loading plugins and authenticated markers; disabling shell_integration removes the shim entirely.
entry_points: Web dock Terminal app; compozy terminal open --shell; [terminal] shell_integration
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: isolated CLI lab walk (fish THEME-OK, walkfn, marker journal rows; zsh clean + marker row); docs/qa/reports/2026-08-31-terminal-stabilization.md
last_report: docs/qa/reports/2026-08-31-terminal-stabilization.md
overlaps: ET-terminal-journal-recording
---

Added 2026-08-31: the shim previously overrode fish's XDG_CONFIG_HOME, hiding
themes ("No such theme: rose-pine"), conf.d, functions, and fish_variables.
Markers now ride fish's vendor_conf.d through XDG_DATA_DIRS.

Walk:

1. With a fish config that sets a theme (for example rose-pine), open a terminal with fish; confirm the theme applies and `fish_config theme show` finds it.
2. Confirm user functions, completions, and universal variables behave exactly as in a stand-alone fish.
3. Run a command and confirm the journal records it with `detected_by: "marker"`.
4. Open interactive and login zsh terminals with ZDOTDIR unset and with a custom directory containing spaces and quotes. Compare with a standalone zsh: `.zshenv`, `.zprofile`, `.zshrc`, `.zlogin` and `.zlogout` run in the same order with the same ZDOTDIR state. Load a bundle through `${ZDOTDIR:-$HOME}/.zsh_plugins.txt` and confirm it loads.
5. Repeat with `.zshenv` setting or unsetting ZDOTDIR, and with `.zprofile` or `.zshrc` redirecting it. Confirm later files follow the changed directory and commands still emit authenticated start/finish markers.
6. Start a nested interactive zsh. Confirm its bundle loads from the user directory and it receives neither the shim path nor the parent's marker nonce.
7. In an isolated startup harness, launch a child between the env/profile and rc bridges, as a global startup file can do. Confirm it restores the user directory/export state and never installs the parent nonce.
8. Set `[terminal] shell_integration = false`; confirm the shell starts with no shim environment and journal rows degrade to `idle` detection.

2026-09-12: Issue #626's real-zsh PTY replay covers the changed startup slice, including
native/integrated trace equality and authenticated command markers. See
[Zsh startup fidelity](../reports/2026-09-12-zsh-startup-fidelity.md). Prior fish and Web evidence
is unchanged; this replay does not claim a new rendered Web or packaged-desktop walk.

9. After restarting the isolated daemon with shell integration disabled, compare fresh zsh terminals
   with `PROMPT=$'%F{blue}%~%f\n%F{magenta}❯%f '` and the identical prompt ending in `>`.
   Type `a`, `b`, and `c` separately; each key must appear before Enter. Compare `terminal quote`
   with the visible screen, reconnect, erase a character, and execute a command. Confirm Unicode
   and ANSI colors survive and input remains responsive.

2026-09-12 QA impact: step 9 owns the Unicode/C1 filtering regression in #629; its targeted replay
is tracked in [the Unicode prompt report](../reports/2026-09-12-terminal-unicode-prompt.md).
Earlier evidence remains applicable to unchanged fish and shell-config loading behavior.

2026-09-12 follow-up: the browser redraw checks reached their quote assertion after CI shell
permissions were repaired. PTY line reads now project the retained bytes through the VT emulator,
applying cursor edits before selecting the quote range. Native CLI/HTTP replay verifies both prompt
glyphs, separate keystrokes, Backspace and subsequent command execution. Browser replay remains
owned by the unchanged E2E cases; see [the integration report](../reports/2026-09-12-pr-630-634-integration.md).
