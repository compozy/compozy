# Proposing a reusable skill

Use a skill for a procedure worth repeating, with a clear trigger and knowledge the agent would otherwise need to rediscover. Keep one-off task state in task notes or memory instead.

## Write the proposal

- Give it a specific name and a short description explaining when to use it.
- Write the smallest procedure that reliably produces the intended result. Include prerequisites, necessary ordering, expected outputs, and meaningful verification.
- Reuse existing tools and conventions. Move optional examples and detailed references into supporting files; link them from the body with the condition for reading them.
- Include scripts only when they make a repeated operation more reliable. Explain their inputs, outputs, and side effects. Never include credentials or session secrets.
- Describe hooks, MCP servers, scripts, and anything that could run automatically so the operator can assess the whole proposal.
- Use evidence from completed work. Do not turn an unverified workaround into a standing rule.

## Lesson or memory?

A skill proposal is a **lesson**: a reviewed, evidence-backed change to a procedure, installed as a skill. Agent memory (`compozy__agent_memory_add`) is a quick, unreviewed note of up to 1,000 characters, optionally expiring with `until`, that you weigh as data; it never changes a procedure.

| Use                                     | When                                                                              | Review   | Lifetime                 |
| --------------------------------------- | --------------------------------------------------------------------------------- | -------- | ------------------------ |
| `compozy__agent_memory_add` (note)      | Context to recall: a host name, a temporary state, a date                         | None     | Until removed or `until` |
| `compozy__agent_skill_propose` (lesson) | The next run of a task should be done differently, and finished sessions show why | Operator | Installed skill          |

Never promote a note into a lesson by copying it, and never copy a lesson waiting for review into memory to act on it before the operator decides. The two share no storage, tool, or review path.

## Submit and inspect

The experimental `compozy__agent_skill_propose` tool accepts `name`, `description`, `body`, optional `files`, a required `lesson`, and required `evidence_session_ids`. Discover its live descriptor for the exact file representation and limits before calling it.

- `lesson` has `cue` (when it applies), `lesson` (what to do differently), `evidence` (what the sessions showed), and optional `confidence` (`low`, `medium`, or `high`; default `medium`; any other value fails with `skill_propose_lesson_invalid`). `cue`, `lesson`, and `evidence` must be non-blank and at most 2,000 characters each (`skill_propose_lesson_required`, `skill_propose_lesson_too_long`).
- `evidence_session_ids` lists 1–20 sessions that show the lesson. Each must exist in the instance's profile and, for a workspace instance, in its workspace (`skill_propose_evidence_required`, `skill_propose_evidence_not_found` naming the first offending session). Duplicates are collapsed.

The daemon validates and verifies the content, redacts it and the lesson text, and stores the draft outside every skill scan root. A proposal report links to the review through `skill_review_id`. Pending drafts are not offered as skills and nothing in them runs. An instance may have at most five pending reviews; `skill_review_limit` means an existing review must be resolved first.

Read the instance's reviews, with their lessons and evidence sessions, with `compozy__agent_skill_reviews`. Submitting the same contents again while the first review is pending returns that review unchanged with `duplicate_of_pending: true`; its lesson and evidence are not replaced. Contents that were already rejected are refused with `skill_propose_previously_rejected`, naming the rejected review, until the contents change. A new proposal whose name is already installed in the instance's skill root is refused with `skill_propose_name_taken`; re-proposing approved contents never opens another review. Do not treat a pending review as approval.

## Change an installed skill (edit mode)

Pass `mode: "edit"` to change a skill already installed in this instance's skill root (the same root approval installs into; other tiers are never read or written). An edit is an **overlay**: `SKILL.md` is regenerated from `name`, `description`, and `body`; `files` adds or replaces text files; `remove_paths` deletes installed files; every other installed file, binary and executable ones included, is kept byte- and mode-exact. A replaced file keeps its installed mode.

- No skill of that name in the root: `skill_review_base_missing` (use `mode: "new"`). A `new` proposal for an installed name: `skill_propose_name_taken` (use `mode: "edit"`).
- The installed skill has a symlink or another non-regular entry, more than 64 files, or more than 1 MiB: `skill_review_base_unsupported`; it can only be edited directly.
- A result identical to the installed skill: `skill_propose_no_change`.
- `remove_paths` naming a file the installed skill does not have, naming `SKILL.md` or a file the proposal also writes, or used with `mode: "new"`: `skill_propose_invalid_remove`.
- While the skill has an unresolved install conflict: `skill_install_conflict`.

The daemon captures the installed contents as the review's base: an unredacted, lossless snapshot kept outside every scan root for as long as the review exists, with its tree digest as `base_digest`. The review shows each path as added, removed, modified, or unchanged, a diff for modified text files, binary files by name and size, and **Runs automatically** for both the resulting tree (kept files count, so a body-only edit of a skill with `scripts/` still reports them) and the installed base. The stored draft records `mode` and `remove_paths`, so the review digest covers them. Proposal identity is the digest plus `base_digest`: rejected contents are refused with `skill_propose_previously_rejected` only against the same base, and may be proposed again once the installed skill changes.

Approval of an edit succeeds only while the installed skill still equals its base. If it changed (or was deleted) after the proposal, approval fails with `skill_review_base_stale` and the review stays pending; reject it and propose again against the current contents. Two pending edits of one base cannot both be approved: after the first, the second is stale.

## Operator decision

Only an operator may approve or reject, through the review UI, CLI, HTTP, or UDS. There is no agent approval tool. Agent-credentialed decisions fail with `skill_review_operator_only`.

The operator inspects the exact contents and automatic execution details before deciding. Approval verifies the draft digest; changed contents fail with `skill_review_stale` and require a fresh review of those contents. A draft whose digest still matches but cannot be installed (its bytes, name or file paths fail verification) fails with `skill_review_invalid`; it can only be rejected.

For a workspace instance, approval installs into the workspace skills root. For a global instance, it installs into the definition's agent-local `skills/` directory, where hooks do not run. A global definition without a source path cannot receive a draft (`skill_review_no_target`). Content refused by verification returns `skill_propose_unverified`.

After approval, the skill behaves like any other skill in its tier. Change it later with an edit proposal.

The operator can revert the latest approval of a skill (operator or automatic): a reverted `new`
skill is removed, and a reverted edit restores the contents it replaced byte- and mode-exact, after
re-verifying the stored base snapshot. Only the install log's current writer can be reverted, and
only while the skill on disk still matches what that approval installed; otherwise revert fails
with `skill_review_revert_conflict` and a `reason`: `superseded` (a newer approval of the skill
exists, from any instance or home; revert that one first), `changed_on_disk`, `legacy` (approved
before this feature), or `target_unavailable`. After a revert the approval it replaced becomes
revertible again. Reverting a reverted review succeeds without changes; a pending or rejected
review fails with `skill_review_stale`. Contents that were reverted can be proposed again. Digests, edits, and
reverts cover file contents and file permission bits; directory permissions and empty directories
are not part of a skill tree and are not preserved.

## Boundary

This approval path applies to proposals submitted through the tool. Skills written directly through file tools retain their existing behavior, including hooks. There is no folder watcher, baseline, or catalog filter enforcing proposal approval on file writes. A proposal never grants permission for unrelated actions.

## Review commands and exact contents

Use the operator CLI to inspect the proposal and decide the digest you reviewed:

```sh
compozy agent skill review list --workspace my-project
compozy agent skill review show <review-id>
compozy agent skill review approve <review-id> --expected-digest <reviewed-digest>
compozy agent skill review reject <review-id> --expected-digest <reviewed-digest>
compozy agent skill review revert <review-id> --expected-digest <reviewed-digest>
```

`revert` takes the review's `digest` (shown as `Digest:`), calls `POST
/api/agent-skill-reviews/<id>/revert`, and prints `Reverted <name>. The skill was removed.` or
`Reverted <name> to its previous contents.`. `GET /api/agent-skill-reviews/<id>` reports
`revert: {allowed, reason}`, the same rule the revert applies.

For a User-scope instance, select `--workspace @user`. Review output shows the lesson (cue,
lesson, evidence, confidence, and evidence sessions), then **Runs automatically**, before the exact
proposed files. For an edit, `show` prints `Base:` beside the digest, the proposed and base
automatic counts, `Files: N modified, N added, N removed`, and a unified diff of every changed file. A review created before lessons existed shows `No lesson recorded` and is decided
as before. `list` accepts `--state pending|approved|rejected|reverted`, `--kind new|edit`, and
`--decided-via operator|auto_approve`; a `--cursor` only continues the filters it was issued for. Agent credentials cannot decide even with `approve-all`.
Approval of a new skill never replaces anything: if a skill with that name now exists, approval
fails with `skill_review_target_exists` and changes nothing; reject the review and ask for an edit.
Every approval and rejection of one skill is serialized, also across CompozyOS homes registered on
the same project. While another process is changing the skill, propose, approve, and reject fail
with `skill_install_busy`; retry in a moment. Each committed install is recorded in
`<skills root>/.skill-installs/<name>.jsonl`, the authority for what was installed by which review.
An install interrupted by a crash is rolled back (or, after its commit point, finished) the next
time any daemon touches the skill or starts. If the skill directory was edited in the middle of an
install, the daemon keeps both versions (your edit at the skill path, the original next to it) and
refuses further approvals of that skill with `skill_install_conflict` until the operator resolves
it. Resolution is operator only (agent credentials get `skill_review_operator_only`):
`compozy agent skill install-conflict list` shows the open conflicts (`GET
/api/agent-skill-install-conflicts`), and `compozy agent skill install-conflict resolve <op>
--keep current|previous` (`POST /api/agent-skill-install-conflicts/<op>/resolve`) keeps the edited
version and discards the original, or restores the original and moves the edited copy to
`<skills root>/.skill-conflict-<op>/`. The choice is recorded before any file moves, so a resolution
interrupted by a daemon stop is listed as `resolving` and finished at the next start. Repeating it
with the same `--keep` returns `already_resolved` and publishes the catalog again; the other
`--keep` fails with `skill_install_conflict_resolving`, and an unknown op with
`skill_install_conflict_not_found`. A conflict resolved with `--keep current` leaves the skill
operator-authored. A repeated approval of an already installed review is safe, writes nothing, and retries catalog
publication after an interrupted response.

The tool represents supporting files as `{path, content}`. Paths are relative; the daemon generates
`SKILL.md` from `name`, `description`, and `body`. The proposal accepts at most 64 supporting files,
20 evidence session IDs, and 1 MiB of encoded draft contents. Never include secrets.
