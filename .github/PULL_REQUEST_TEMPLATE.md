<!--
Title format: <type>: <description> — type is one of feat | fix | refactor | perf | docs | test | build | ci.
  Good: fix: drop stale session sockets on daemon restart
  Bad:  Fixed a bug · update stuff · fix #123
-->

## What & why

<!-- What changes for someone using CompozyOS, and why.
     Link the issue if there is one: Fixes #123 -->

## How you verified it

<!-- Commands you ran and what they showed. Screenshots or a short clip for UI changes.
     If you added no tests, say why. -->

## Impact

<!-- Does this change a CLI command, an HTTP/UDS route, a config.toml key, or a web surface?
     Name which, and whether the docs (packages/site) need an update.
     "No impact" is a fine answer — just say it. -->

---

- [ ] `make gate` passes locally; this PR is delivered only after its required CI checks are green
- [ ] New or changed behavior is covered by tests, or I explained above why not
- [ ] If an agent wrote or co-wrote this, I named it above and verified the result myself

<!-- First time here? See .github/CONTRIBUTING.md. For anything bigger than a bug fix,
     open an issue first — it protects your time as much as ours. -->
