---
name: apply-prototype-comments
description: Apply gallery review comments to their owning views and verify the commented states.
---

# Apply prototype comments

Read project instructions and docs/design/prototyping.md. Use `proto list --json` and
`proto comments list --status open --json`, then `proto comments show <id> --json` for each
assigned thread. Add `--app <dir>` when needed. Read the whole thread, including attachments:
`proto comments attachments <id> --out .tmp/review-images`. Never edit review JSON directly.

Use context.state, theme, role, viewport and commit to reproduce the review. Source paths are
app-relative; target.sourceChain points from the reviewed element to its composing prototype.
Change the owning view for component prototypes; edit drafts directly. Preserve library tokens,
components and production boundaries. Flow comments identify a node or the whole flow.

Make the smallest change that satisfies the request. Run `proto check --json` and the owning
view's checks, then inspect `proto url <slug> --state <id> --bare` at the reviewed width/theme.
Save `proto screenshot <slug> --state <id>` or `proto evidence <slug> --state <id>` when useful.
Reply with `proto comments reply <id> --note "..."`; resolve only after verification with
`proto comments resolve <id> --note "..."`. Use `proto comments reopen <id>` for a comment that
still needs work. Edit your own messages with `proto comments edit <id> --message <n> --note "..."`.
Attach evidence with `proto comments attach <id> <file>`. Ambiguous requests stay open with a reply.

Agents never approve or reopen reviews, and never delete comments on their own initiative.
Report resolved and open threads with verification evidence. A comment resolution is not approval.
