---
id: ET-cli-shell-completion
area: ET
title: Complete CLI identifier values from the daemon
persona: Ada
journey: J-operate-cli-by-completion
expected: `compozy completion <shell>` scripts resolve live values for identifier arguments and flags; completion returns candidates with descriptions, exits 0, prints no warnings or errors to stderr (Cobra's directive line is the only stderr output), never starts the daemon, never mutates state, keeps default file completion for path arguments, and returns only refs for vault secrets.
entry_points: compozy completion bash|zsh|fish|powershell; hidden compozy __complete
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits: c86faa290
evidence: local CLI walk 2026-11-26 with the worktree build: `__complete session inspect ""` listed live session IDs with titles; `__complete session list --workspace ""` listed workspace names with root dirs; `__complete session list --profile ""` listed profiles; `__complete workspace add ""` returned `:0`; `__complete session inspect sess-` returned `:4`; `completion zsh` emitted `#compdef compozy`.
last_report:
overlaps:
---

QA impact 2026-11-26: new value completion surface. Walk with a running daemon and an isolated
profile: `compozy __complete session inspect sess-` lists matching session IDs with titles and
prints `:4`; `compozy __complete session list --workspace ""` lists workspace names; `compozy
__complete provider models set <provider> ""` lists that provider's models; `compozy __complete
workspace add ""` prints `:0` and no candidates; `compozy __complete daemon profile ""` keeps the
static cpu/heap/alloc profile values; `compozy __complete vault get ""` lists refs with namespace
and kind but never values. Repeat with the daemon stopped: every path exits 0, prints only a
directive line on stdout, and writes no warnings or errors to stderr beyond Cobra's own directive
note. Confirm `compozy completion zsh` still generates a script and that sourcing it does not run
the daemon.
