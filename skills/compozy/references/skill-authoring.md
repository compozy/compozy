# Proposing a reusable skill

Use a skill for a procedure worth repeating, with a clear trigger and knowledge the agent would otherwise need to rediscover. Keep one-off task state in task notes or memory instead.

## Write the proposal

- Give it a specific name and a short description explaining when to use it.
- Write the smallest procedure that reliably produces the intended result. Include prerequisites, necessary ordering, expected outputs, and meaningful verification.
- Reuse existing tools and conventions. Move optional examples and detailed references into supporting files; link them from the body with the condition for reading them.
- Include scripts only when they make a repeated operation more reliable. Explain their inputs, outputs, and side effects. Never include credentials or session secrets.
- Describe hooks, MCP servers, scripts, and anything that could run automatically so the operator can assess the whole proposal.
- Use evidence from completed work. Do not turn an unverified workaround into a standing rule.

## Submit and inspect

The experimental `compozy__agent_skill_propose` tool accepts `name`, `description`, `body`, optional `files`, and optional `evidence_session_ids`. Discover its live descriptor for the exact file representation and limits before calling it.

The daemon validates and verifies the content, redacts it, and stores the draft outside every skill scan root. A proposal report links to the review through `skill_review_id`. Pending drafts are not offered as skills and nothing in them runs. An instance may have at most five pending reviews; `skill_review_limit` means an existing review must be resolved first.

Read the instance's reviews with `compozy__agent_skill_reviews`. A rejected draft stays rejected until its contents change. Do not resubmit unchanged rejected content or treat a pending review as approval.

## Operator decision

Only an operator may approve or reject, through the review UI, CLI, HTTP, or UDS. There is no agent approval tool. Agent-credentialed decisions fail with `skill_review_operator_only`.

The operator inspects the exact contents and automatic execution details before deciding. Approval verifies the draft digest; changed contents fail with `skill_review_stale` and require a fresh review of those contents.

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

For a User-scope instance, select `--workspace @user`. Review output shows **Runs automatically**
before the exact proposed files. Agent credentials cannot decide even with `approve-all`.
Approval never replaces existing different contents. A repeated approval of the same installed
contents is safe and retries catalog publication after an interrupted response.

The tool represents supporting files as `{path, content}`. Paths are relative; the daemon generates
`SKILL.md` from `name`, `description`, and `body`. The proposal accepts at most 64 supporting files,
20 evidence session IDs, and 1 MiB of encoded draft contents. Never include secrets.
