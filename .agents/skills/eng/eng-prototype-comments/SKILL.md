---
name: eng-prototype-comments
description: Apply review comments left in the Compozy prototype gallery (`bun run proto`) to the real web views or drafts they point at, verify the commented state, and resolve or reply through the proto CLI. Use for gallery comment ids (`<slug>-<n>`) or requests to process prototype review comments. Do not use for writing new prototypes, Storybook stories, or GitHub PR review threads.
---

# Apply Prototype Comments

Reviewers comment in the gallery at `http://127.0.0.1:4400` by clicking an element or dragging an
area of a prototype. Each comment records where it points and what the reviewer saw. Apply the note
in the code that owns that pixel, verify the exact view, then close the loop with
`bun run proto comments`. Run every command from the repository root.

Comments live in `web/gallery/comments/<slug>/comments.json` with images beside them in
`attachments/`. The CLI reads and writes those files with or without the dev server; a running
gallery picks up changes live. Never edit the JSON by hand and never delete a comment: deletion is
the reviewer's call.

## Commands

```bash
bun run proto list --json                       # slug, kind, file, states, open comment count
bun run proto comments list [--slug <slug>] [--status open|resolved|all] [--json]
bun run proto comments show <id> [--json]
bun run proto comments attachments <id> --out <dir>
bun run proto comments reply <id> --note "<question or status>"
bun run proto comments resolve <id> --note "<what changed, which files, how verified>"
bun run proto comments reopen <id> [--note "<why>"]
bun run proto comments edit <id> --note "<corrected text>" [--message <n>]
bun run proto comments attach <id> <file…> [--message <n>]
bun run proto comments detach <id> <attachment-id>
```

`--author` defaults to `agent`. `--message` is one-based; without it, `edit` and `attach` target
your latest message. Edit only your own messages, never the reviewer's request.

## Procedure

**Step 1: Read the comments in scope**

1. `comments list` (open by default), grouped by slug; `proto list --json` once for each slug's
   file, kind and states.
2. `comments show <id>` for each. `thread[0]` is the request; later messages are discussion. Read
   the whole thread.
3. Download attachments with `comments attachments <id> --out "$(mktemp -d)"` and look at every
   image before deciding what a visual note asks for.

*Done when:* you can state, per comment, the requested change and the exact view it applies to.

**Step 2: Locate the owning source**

1. `context` is the view: `state`, `theme` and `viewport` width × height. A note at 390 px or in
   light theme may concern only that combination.
2. `target.source` is the repo-relative JSX location (`web/src/…tsx:<line>:<col>`) of the clicked
   element or the center of a region; `sourceChain` lists more locations, most specific first,
   including component calls from the prototype. Use `selector`, `testId` and `text` when no
   source tag is present.
3. If `context.commit` differs from `HEAD`, `show` prints `git show <commit>:<file>`; read that
   revision first, since line numbers may have moved.
4. Classify the prototype:
   - **Component prototype** (`web/src/**/*.proto.tsx`): renders real views with fixtures. Change
     the owning view in `web/src/systems/**`, not the prototype. A shared view change also
     affects production and other prototypes: check their callers.
   - **Draft** (`web/src/drafts/<slug>/index.proto.tsx`): free markup; edit the draft itself.
5. Flow comments use the key `flow-<id>` with a `node` target (no source). They concern the
   journey file under `web/gallery/flows/`; for a screen-level note, open the node's prototype.

*Done when:* each comment maps to one file and element you are confident owns it.

**Step 3: Make the smallest change**

1. Read `DESIGN.md` for visual grammar and `COPY.md` for wording before editing; load
   `eng-design` for any visual change. UI copy is English; tokens own colors, type and spacing.
2. Change only what the note asks, in the comment's state, theme and width. Compose
   `@compozy/ui` primitives; do not change `packages/ui` for a prototype note without the
   user's authorization; reply on the comment instead.
3. Keep fixtures in fixture files and data access out of prototypes (`proto check` enforces it).

*Done when:* the diff is the minimum that satisfies every note in scope.

**Step 4: Check and verify**

1. `bun run proto check`, then the targeted lanes:
   `bunx turbo run typecheck lint --filter=./web` and the owning test suite of a changed view.
2. `bun run proto screenshot <slug> --state <state> --theme <theme> --width <width>` at the
   comment's view, and open the PNG. It reuses a running `proto dev` or starts one for the run.
3. When the change is visible in other widths or themes the note did not mention, capture those
   too and confirm nothing regressed.

*Done when:* checks are green and a screenshot of the commented view shows the change.

**Step 5: Close the loop**

1. Resolve only after the change exists and was verified:
   `comments resolve <id> --note "<what changed, files, screenshot path>"`.
2. For ambiguity, conflicting notes, or work outside the authorized scope (including
   `packages/ui`), `reply` with the concrete question and leave the comment open.
3. Report resolved and open ids with the checks you ran. Do not commit unless asked.

*Done when:* every comment in scope is resolved with a note or has a reply explaining why not.

## Error Handling

- **`Unknown prototype "<slug>"`:** the prototype was renamed or removed; find it with
  `proto list` and reply on the comment rather than guessing.
- **`attachment_limit`:** a message takes up to 6 images of 5 MB (PNG, JPEG, GIF, WebP).
- **The pin shows "anchor not found":** the DOM changed since the comment; locate the element
  from `source`, `text` and the commit's file, not from the stale selector.
