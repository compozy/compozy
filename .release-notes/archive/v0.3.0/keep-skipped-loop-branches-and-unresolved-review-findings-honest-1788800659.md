---
title: Keep skipped Loop branches and unresolved review findings honest
type: fix
---

Exclusive routing no longer executes a dominated branch later in the same planning pass. Skipped downstream and fan-out steps settle as Not taken instead of appearing permanently pending after the run ends; work that ran in earlier rounds keeps its history.

Review-and-fix findings that are valid but blocked or unresolved remain pending and retain their status. Invalid findings remain invalid. Finalization no longer counts those findings as resolved simply because the review round finished.

PRs: [#529](https://github.com/compozy/compozy/pull/529), [#543](https://github.com/compozy/compozy/pull/543), [#544](https://github.com/compozy/compozy/pull/544).
