---
name: proto-apply-comments
description: Apply @compozy/proto gallery review comments to the code that renders them, verify the reviewed state and answer each thread. Use for a comment id such as `login-form-7` or `flow-checkout-2`, a gallery link with `comment=`, or a request to address open prototype review comments. For authoring prototypes or setting up proto, use the proto skill.
---

# Apply prototype review comments

A reviewer pinned a comment to something rendered in the proto gallery. The comment records where
it was clicked in the source, plus the exact state, theme, role, viewport and commit they saw.
Your job is to change the code that ships, prove the reviewed state now looks right, and answer
the thread. The `proto` skill installed beside this one owns the concepts and hard rules: read
its `SKILL.md` when prototypes, drafts, flows or guards are unfamiliar, `references/cli.md` for a
command's flags or JSON, and `references/troubleshooting.md` when a command fails. Project rules
in `AGENTS.md` or `CLAUDE.md` take precedence for code conventions.

## Rules

- **Fix the owning view, not the prototype.** A component prototype only composes a view with
  fixtures, so the reviewed pixels come from the view, and that is what ships. Edit a draft
  directly, a flow in its `.flow.ts`, and fixture data only when the comment is about the data.
- **Agents never approve.** Do not run `review approve`, `review reopen` or `review unlock`, and
  delete a comment only when a person asks. Resolving a thread is not approval.
- The `proto` hard rules still apply: never edit `proto/review/**` by hand (write through
  `proto comments ...` or MCP), never weaken a guard, and never copy view markup into a
  `.proto.tsx` to satisfy a comment.

## Loop

Add `--app <dir>` to every command in a monorepo. When the review lives on the deployed gallery,
add `--remote` to the `comments` and `review status` commands only. `check`, `flow show` and
`screenshot` always run locally and exit 2 with `--remote`. The MCP tools (`proto_comments_list`,
`proto_comments_show`, ...) take the same fields and return the same JSON.

1. **Find the threads.** For a given id, run `proto comments show <id> --json`. Otherwise run
   `proto comments list --json` (open by default; filter with `--slug <key>`, `--spec`,
   `--state`). Threads whose last message is from an agent are waiting on a person
   (`--waiting`); leave them alone unless something changed.
2. **Read everything.** Read every message in the thread, not just the first, because later
   replies often change the request. Download the images with
   `proto comments attachments <id> --json` (into `.tmp/proto/<id>/`) and look at them.
3. **Locate the source.**
   - `target.source` is the app-relative `path:line:column` of the JSX nearest to the clicked
     element. `target.sourceChain` walks outward from it to the composing prototype. The first
     entry outside the `.proto.tsx` is usually the owner. If that entry is a shared component
     and the request is about this screen only, the fix belongs at the call site further out in
     the chain. When only the `.proto.tsx` appears, the comment is about fixtures or state
     selection.
   - `target.text`, `selector` and `testId` confirm the element. A `region` target lists what
     was drawn inside the dragged area.
   - Lines refer to `context.commit`. If the file changed since, find the element by its text or
     with `git diff <commit> -- <path>`.
   - An empty chain means the element came from outside the app's sources (for example a
     design-system package). The owner is the app view that renders that component.
   - Flow comments have `target.kind: "node"`. `node` is the flow node id, or `null` for the whole
     flow. `proto flow show <id> --json`, where `<id>` is the comment key without its `flow-`
     prefix, maps the node to its prototype and state.
4. **Reproduce.** With `proto dev` running, open the gallery URL from
   `proto url <slug> --state <context.state> --theme <theme> --role <role> --width
   <viewport.width>`, leaving out the flags whose context value is `null`. The gallery applies
   the width; a `--bare` render document takes the window's width instead. For a flow comment,
   use the node's prototype and state.
5. **Fix** the owner. When the request needs a state the prototype lacks, add it to the view's
   props and to the prototype's `meta.states`.
6. **Verify.** `proto check --json` must exit 0, with no `problems` and no `error`. Run the
   view's own checks (typecheck, stories, tests). Look at the reviewed state at the reviewed
   theme, role and width, for example with
   `proto screenshot <slug> --state <id> --theme <theme> --width <w> [--role <role>]`. If the view
   appears in other prototypes or states, check those too.
7. **Answer.**
   - Done and verified: `proto comments resolve <id> --note "<what changed, where, how you
     verified it>"`. You can then `proto comments attach <id> <png>`, which attaches the image to
     that note.
   - Ambiguous, needs a product decision, or out of scope: `proto comments reply <id> --note
     "<question or reason>"` and leave the thread open.
   - A resolved thread that still needs work: `proto comments reopen <id> --note "..."`. That
     reopens the comment; reopening the review is a person's call.
   - Fix your own wording with `proto comments edit <id> --note "..." [--message <n>]`.

The work is done when every thread you took is either resolved with a verified note or open with
a reply that says what is needed. Approval stays with the reviewer.

## Pitfalls

- `resolve` on a thread that is already resolved changes nothing, and its `--note` is dropped.
  Use `reply` instead.
- `edit`, `attach` and `detach` only work on your own messages (`message_not_owned`).
- `--author` defaults to `agent`. Never post as `reviewer`.
- `comments add` on a prototype key needs `--selector` (`selector is required`); flow keys do
  not. Agents add comments only when a person asks for one.
