---
name: proto-apply-comments
description: Apply @protohq/cli gallery review comments to the code that renders them, verify the reviewed state and answer each thread. Use for a comment id such as `login-form-7` or `flow-checkout-2` or `story--ui-button-3`, a gallery link with `comment=`, or a request to address open prototype review comments. For authoring prototypes or setting up proto, use the proto skill.
---

# Apply prototype review comments

A reviewer pinned a comment to something rendered in the proto gallery. The comment records where
it was clicked in the source, plus the exact state, theme, role, viewport and commit they saw.
Your job is to change the code that ships, prove the reviewed state now looks right, and answer
the thread. The `proto` skill installed beside this one owns the concepts and hard rules: read
its `SKILL.md` when prototypes, drafts, flows or guards are unfamiliar, `references/cli.md` for a
command's flags or JSON, and `references/troubleshooting.md` when a command fails. Project rules
in `AGENTS.md` or `CLAUDE.md` take precedence for code conventions.

Local `comments *`, `screenshot`, `evidence` and every local MCP tool call are Pro
operations. Proto Cloud checks each operation online, including in CI. An Editor seat in a
Pro workspace covers unlimited operations. Without a seat, the account has 100 free
operations once, with no reset.

## Rules

- **Fix the owning view, not the prototype.** A component prototype only composes a view with
  fixtures, so the reviewed pixels come from the view, and that is what ships. Edit a draft
  directly, a flow in its `.flow.ts`, and fixture data only when the comment is about the data.
- **Agents never approve.** Do not run `review approve`, `review reopen` or `review unlock`, and
  delete a comment only when a person asks. Resolving a thread is not approval.
- Never read or edit `proto/review/**` directly or call the gallery's `/__proto/api`. Use the
  CLI or MCP. Pinning an older CLI or setting `CI` to bypass access checks is prohibited.
- Relay allowance notices from CLI stderr or the trailing `[Proto]` MCP item once per stage:
  first use, low, and used up. On `plan_required` or `seat_required`, stop the paid step, quote
  the error message and follow `next` with its URL: upgrade, ask an owner to upgrade, or ask
  an owner for an Editor seat. Continue with Free CLI commands such as `proto check` or
  `proto url`, or direct the person to the gallery for human review. If you only have MCP,
  stop using Proto tools until access is restored. Every MCP tool call is Pro.
- Preserve guards and keep the owning view's markup out of the `.proto.tsx`.

## Loop

Add `--app <dir>` to every command in a monorepo. When the review lives on the deployed gallery,
use `--remote --share <slug>` on `comments` and `review status` for hosted review access. `check`, `flow show` and
`screenshot` always run locally and exit 2 with `--remote`. The MCP tools (`proto_comments_list`,
`proto_comments_show`, ...) take the same fields. Their first text item is the same JSON as
the CLI result. A trailing `[Proto]` allowance notice may follow.

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
4. **Reproduce.** Select the owning app with `--app` for project commands;
   `proto status --json` lists projects without starting anything. Run `proto dev` in that app's terminal and open its `/p/<label>/` gallery
   URL from
   `proto url <slug> --state <context.state> --theme <theme> --role <role> --width
   <viewport.width>`, leaving out the flags whose context value is `null`. The gallery applies
   the width; a `--bare` render document takes the window's width instead. For a flow comment,
   use the node's prototype and state. Get preview addresses from `proto url <slug> --bare` after
   the first start, never by constructing a port. The home at `http://127.0.0.1:4400/` and the
   project menu switch projects; comment drafts are isolated per project.
5. **Fix** the owner. When the request needs a state the prototype lacks, add it to the view's
   props and to the prototype's `meta.states`.
6. **Verify.** `proto check --json` must exit 0, with no `problems` and no `error`. Run the
   view's own checks (typecheck, stories, tests). Look at the reviewed state at the reviewed
   theme, role and width, for example with
   `proto screenshot <slug> --state <id> --theme <theme> --width <w> [--role <role>]`. If the view
   appears in other prototypes or states, check those too. If capture is unavailable, use
   the Free preview URL from step 4 to inspect the same state and report what you verified.
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

## Local process lifecycle

Ctrl-C stops only this project's `proto dev`. `proto stop --json` stops the shared Proto process
and leaves project terminals waiting; `proto start --json` reconnects them. Both are idempotent;
`proto status --json` starts nothing. `PROTOHQ_PORT` chooses Proto's remembered port (default
4400); stop it before moving the address. `proto dev --json` prints one stdout JSON line and
keeps serving; later status messages go to stderr. Captures reuse a verified ready project even
while Proto is stopped, or temporarily serve it without starting Proto in the background.

For failures, inspect `~/.config/protohq/daemon/daemon.log` and the sibling proto skill's
`references/troubleshooting.md`: `port_in_use`, `daemon_unavailable`, `daemon_version_mismatch`,
`state_unavailable`, `project_busy` and `project_offline` have specific recovery steps.

## Pitfalls

- `resolve` on a thread that is already resolved changes nothing, and its `--note` is dropped.
  Use `reply` instead.
- `edit`, `attach` and `detach` only work on your own messages (`message_not_owned`).
- `--author` defaults to `agent`. Never post as `reviewer`.
- `comments add` on a prototype key needs `--selector` (`selector is required`); flow keys do
  not. Agents add comments only when a person asks for one.

## Component story comments

A `story--<component slug>` key addresses a CSF file; `context.state` is its story id.
Use `proto stories list --filter '<component slug>--*' --json` to find that file and
`proto stories url <context.state>` to reopen the story. Apply theme and viewport from the
comment in the gallery. Fix the owning component or its fixtures as appropriate, preserving
CSF and preview setup. Verify with `proto stories run --filter '<story-id>' --json` plus the
project's checks and a visual review in `proto dev`; prototype screenshot/evidence commands
are not story capture commands. Reply/resolve with the ordinary comment commands.

Read threads with `proto comments list --slug story--<slug> --json`. Adding a component comment
requires `--state <story-id>` belonging to the component (`invalid_input` if absent/invalid),
with the normal `--note` and `--selector`. Existing `story-*` prototype slugs remain prototypes.
Story review is local; do not pass `--remote`. Agents never approve component reviews.

## Hosted review

For a hosted share, run `proto login` and include `--remote --share <slug>` in store commands.
Use `proto comments list --remote --share <slug> --json` to find threads. Keep the share explicit
for every operation. Remote MCP can reply, resolve and reopen; approval remains a human browser
action. Read `../proto/references/hosted.md` for setup and `../proto/references/errors.md` for errors.
