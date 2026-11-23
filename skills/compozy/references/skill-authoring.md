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

## Operator decision

Only an operator may approve or reject, through the review UI, CLI, HTTP, or UDS. There is no agent approval tool. Agent-credentialed decisions fail with `skill_review_operator_only`.

The operator inspects the exact contents and automatic execution details before deciding. Approval verifies the draft digest; changed contents fail with `skill_review_stale` and require a fresh review of those contents. A draft whose digest still matches but cannot be installed (its bytes, name or file paths fail verification) fails with `skill_review_invalid`; it can only be rejected.

For a workspace instance, approval installs into the workspace skills root. For a global instance, it installs into the definition's agent-local `skills/` directory, where hooks do not run. A global definition without a source path cannot receive a draft (`skill_review_no_target`). Content refused by verification returns `skill_propose_unverified`.

After approval, the skill behaves like any other skill in its tier. It is not tracked for further reviews.

## Boundary

This approval path applies to proposals submitted through the tool. Skills written directly through file tools retain their existing behavior, including hooks. There is no folder watcher, baseline, or catalog filter enforcing proposal approval on file writes. A proposal never grants permission for unrelated actions.

## Review commands and exact contents

Use the operator CLI to inspect the proposal and decide the digest you reviewed:

```sh
compozy agent skill review list --workspace my-project
compozy agent skill review show <review-id>
compozy agent skill review approve <review-id> --expected-digest <reviewed-digest>
compozy agent skill review reject <review-id> --expected-digest <reviewed-digest>
```

For a User-scope instance, select `--workspace @user`. Review output shows the lesson (cue,
lesson, evidence, confidence, and evidence sessions), then **Runs automatically**, before the exact
proposed files. A review created before lessons existed shows `No lesson recorded` and is decided
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
it. A repeated approval of an already installed review is safe, writes nothing, and retries catalog
publication after an interrupted response.

The tool represents supporting files as `{path, content}`. Paths are relative; the daemon generates
`SKILL.md` from `name`, `description`, and `body`. The proposal accepts at most 64 supporting files,
20 evidence session IDs, and 1 MiB of encoded draft contents. Never include secrets.
