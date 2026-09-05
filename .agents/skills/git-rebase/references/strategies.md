# Rebase Strategy Selection

- Default: replay the branch onto the requested base, preserving the authored commit sequence.
- Interactive rebase: use for an explicitly requested reorder, squash, drop, or message change. Do not select it from an arbitrary commit-count threshold.
- Repeated resolutions: an existing `rerere` configuration may help, but inspect its result. If enabling it for this operation, prefer invocation/repository scope; do not change global preferences without authorization.
- Shared branches: confirm the request covers rewriting that branch and account for concurrent updates. Do not silently replace an explicitly requested rebase with a merge.
- Validate the final changed behavior using the project's scoped checks and delivery rules. A full suite per commit/file adds no evidence when those inputs did not change.

In rebase conflicts the side labels differ from an ordinary merge: `ours` is the rebased target state; `theirs` is the replayed branch commit. See the [Git rebase manual](https://git-scm.com/docs/git-rebase#Documentation/git-rebase.txt---merge).
