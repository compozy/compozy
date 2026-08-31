---
id: ET-terminal-shell-config-fidelity
area: ET
title: A terminal shell keeps the user's own configuration
persona: Marina
journey: J-operate-integrated-terminal
expected: fish opens with the user's themes, functions, completions, and universal variables intact while journal markers still record commands; zsh sources the user's .zshenv and .zshrc; disabling shell_integration removes the shim entirely.
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
4. Open a zsh terminal; confirm values exported from the user's `.zshenv` are present and `.zshrc` ran.
5. Set `[terminal] shell_integration = false`; confirm the shell starts with no shim environment and journal rows degrade to `idle` detection.
