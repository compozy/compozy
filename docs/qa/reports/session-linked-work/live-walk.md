# Session linked work: live walk

Date: 2026-11-02. Build: branch binary and web bundle at `3cd4025e0`, in an isolated lab from `eng-qa-bootstrap` (targeted profile: CLI, API, Web, runtime, provider). Lab teardown was clean. Provider: Claude Sonnet 5.5. Forge: the bundled GitHub provider against the real `compozy/compozy` repository, read-only. Nothing was pushed from the lab.

## Walked live

| Journey | Result |
| --- | --- |
| A session calls `compozy__worktree_create` mid-turn | The worktree appears on the session as `created`, with no bind and no restart |
| The agent calls `compozy__session_link` for PR #1160 | Listed as "Added by General"; status, title and failing check come from the real forge |
| Link by number from HTTP and the dialog | #1147, #1156, #1158 and #1159 resolve with their titles |
| Unknown number | Empty result, "Can't find" state |
| Another project's PR | 400 `session_link_repository_mismatch`; the dialog explains it |
| Invalid candidate kind | 400 `session_link_invalid` |
| Child session linking #1157 | Reads "From Child with a PR" on the parent |
| Unlink, then Undo | Row leaves and returns; `+4 PRs` then `+5 more`; the toast states nothing was deleted |
| Kanban and list | Card shows `#1160 +5 more`; rows show the count and needs-you dot |
| Narrow window | The count entry appears in the head's actions when the status zone clips |
| Light and dark | Popover and dialog |

## Board column, sampled every 2 seconds

The session held an open linked PR. Two prompts ran, the second delivered during the first.

| Moment | Column |
| --- | --- |
| Stopped, before the prompts | In review |
| Turn running (6 samples) | Working |
| Waiting on a live child (4 samples) | Waiting |
| Child stopped, session idle | In review |

It never reached In review while working.

## Found by this walk and fixed

- Search for another project's PR returned 502 instead of the refusal: `fix: refuse invalid link candidates with their stable codes`.
- The list popover stayed open over the link dialog: `fix: close the linked work list when the dialog opens`.
- The native link tool did not return the PR number, reported by the lab's agent. Fixed in the same commit.

## Not walked live

- The palette, except through its focused suites and proto states.
- Worktree removal with a live PR, and a disconnected forge. Covered by automated tests and proto states.
- 50 real links. Covered by the proto state.
- The `dispatching` queue state between turns at the column level: the second prompt was steered into the live turn, so it never queued. Covered by the Go and SQL matrix.
- E2E-006, which runs in CI.

Candidate search took about 4 seconds against real GitHub, under machine load.
