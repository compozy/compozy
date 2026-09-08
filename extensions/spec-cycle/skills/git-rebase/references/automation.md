# Automated Conflict Resolution

Automatic resolution is appropriate only when the desired semantics are established and its result is inspected. Prefer Git's ordinary merge machinery over blanket side selection.

In a rebase, `-X ours` favors the rebased target state at conflicting hunks; `-X theirs` favors the replayed commit. Neither proves the merged behavior is correct. Do not use these flags as a generic retry after conflict, or confuse strategy (`-s`) with strategy option (`-X`). The previous `-Xrecursive` recipe was invalid.

Use the [Git rebase manual](https://git-scm.com/docs/git-rebase) for supported options of the installed Git version. Preserve authorized history, compatibility obligations, and required final checks.
