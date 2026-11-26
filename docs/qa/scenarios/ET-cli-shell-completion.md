---
id: ET-cli-shell-completion
area: ET
title: Complete CLI identifier values from the daemon
persona: Ada
journey: J-operate-cli-by-completion
expected: `compozy completion <shell>` scripts resolve live values for identifier arguments and flags; completion returns candidates with descriptions, exits 0, prints nothing to stderr, never starts the daemon, never mutates state, keeps default file completion for path arguments, and returns only refs for vault secrets.
entry_points: compozy completion bash|zsh|fish|powershell; hidden compozy __complete
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
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
directive line, and writes nothing to stderr. Confirm `compozy completion zsh` still generates a
script and that sourcing it does not run the daemon.
