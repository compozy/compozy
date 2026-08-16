# Session attachments — shared contract

Issue: [#366](https://github.com/compozy/compozy/issues/366) — paste, drag-and-drop, and file picker on the session composer.
Copy authority: `COPY.md` (prefer **session** over chat) + issue vocabulary (**attachment**, not section).
Every file links `../design-system/ds-core.css` then `attach.css`. Lucide via CDN + `createIcons()`.

## Shared data story

Workspace **compozy**. Session **Calm transcript migration** (claude-code · Claude Sonnet). The operator is sending screenshots of the loud UI they want flattened, plus the spec and notes that go with them.

| file (record label) | origin | MIME | bytes | px | state |
| --- | --- | --- | --- | --- | --- |
| goal-chip.png | picker | image/png | 240 KB | 1840×920 | ready |
| permission-dock.png | drop | image/png | 96 KB | 1280×720 | ready |
| Screenshot 2026-08-13 at 10.28.png | paste | image/png | 188 KB | 1472×918 | ready |
| agents-list.png | picker | image/png | 312 KB | 1840×920 | ready |
| markers-gallery.png | picker | image/png | 268 KB | 1472×918 | ready |
| empty-session.png | drop | image/png | 194 KB | 1280×720 | ready |
| notes.md | picker | text/markdown | 4 KB | — | ready |
| flatten-spec.pdf | drop | application/pdf | 186 KB | — | ready |
| error-log.txt | picker | text/plain | 12 KB | — | ready |
| mockup-4k.png | picker | image/png | 18.2 MB | 3840×2160 | error · too large |
| scan-desk.jpg | picker | image/jpeg | — | — | error · persist failed |
| diagram.svg | picker | image/svg+xml | 12 KB | — | rejected |
| archive.zip | drop | application/zip | 2.1 MB | — | rejected |
| deck.pptx | picker | application/vnd… | 4.8 MB | — | rejected |

v1 MIME: **PNG, JPEG, WebP** (issue) **+ PDF, Markdown, plain text** (this set). GIF / SVG / ZIP / Office / binaries are refusals.

ACP mapping is not shipped. Tag honestly:

- Images → `ContentBlockImage` · `new · prompt-images`
- PDF → embedded resource blob · `new · prompt-files`
- MD / TXT → extra prompt text *or* resource; UI still shows a file tile · `new · prompt-files`

## Locked decisions (apply everywhere)

- **Draft attachments live inside the composer field**, above the textarea. Docks and queued strips stay *above* the field. Drafts disappear with send; they are not a second dock.
- **One tile grammar.** 36×36 well (`radius-md`) · filename · mono size · 22px ×. Images fill the well with a photo. Files fill it with a 9px mono extension (`PDF` / `MD` / `TXT`) — not a Lucide file glyph, not a 22px chip, not a fake document preview.
- **Many items stay one row.** Overflow is a horizontal track with edge fades — workspaces anatomy (pinned edges, hidden scrollbar), not a second line, not a carousel, not `+N`. Count = tiles. The fade dissolves into the **host fill** (`--att-fade-bg`: composer `--elevated`, transcript `--canvas`) with the same RGB and falling alpha. Do not copy the workspaces `rgba(0,0,0,.28)` veil — that is for glass over wallpaper; on a solid field it reads as a black strip. `attach.js` hydrates `.att-strip` / `.msg-user__atts` into `att-rail__track` + pinned edges, paints `data-overflow="start|end|none"`, and maps vertical wheel to `scrollLeft`. No grab-drag (the composer still accepts file drops). Focused × / frame scrolls clear of the fade via `scrollTo`, never `scrollIntoView`. Transcript frames keep 280px; density is scroll, not `data-dense`.
- **Entry points:** (1) paste when the clipboard holds an image, (2) drop on the composer root, (3) 26×26 paperclip `att-btn` after `runtimeControl`. `aria-label="Attach"`. No `/image` or `/attach` slash command — `/` is skills. Network `/attach` (URL / capability ref, post-MVP) is a different product.
- **Drop overlay** covers the composer root only. Copy: **Drop files** / `Images, PDF, Markdown, or text`. Dashed inner stroke + `border-color: var(--accent-dim)` on the field.
- **Send is the only primary.** Attach is a ghost icon button. Accent budget: send disc + (when dragging) the drop border.
- **Mixed strips are one list.** Images and files sit in the same `att-strip`, one row. Send waits until every tile is `ready` (no rejected / uploading / error).
- **Capability gate** for images is a functional label *inside the strip*. Files do not use that row — a text-only model can still take MD/TXT as prompt text; PDF follows `new · prompt-files`. Never silently drop a tile.
- **Persist before accept.** Tiles start `uploading`; Send stays disabled until every tile is `ready`. Persist failure keeps the draft and offers **Retry**.
- **Transcript:** images sit *above* the text bubble as frames (outside the 176px clamp). Files sit in the same gallery as `.att-file` cards (well + name + size, no ×). Image-only / file-only turns skip the empty bubble. Hairline, no accent border.
- **Busy session:** attachments stay on the draft. A queued row may show one 16px well + `· N images` / `· N files` / both. Queue payload is `new · prompt-images` / `new · prompt-files`.
- **Empty session:** no permanent “drop files here” subtitle. Overlay only while dragging.
- **Unsupported types** refuse in place (tile `rejected`). Reason names the allowed types. They never become a chip.
- **Truthful UI:** prompt POST, queue, steer, and ACP are text-only today. Boards that send attachments are tagged. Do not draw upload routes or `input_modalities` badges as if they shipped.
- **Name is the original filename** (picker/drop) or the OS paste label. Size is mono. Dimensions live in `title` on images only.
- Color = state. Uploading = spinner. Error/rejected = danger text. Ready = neutral. Accent never paints a tile border. File extension marks stay `--subtle` (danger only when rejected).
- Gating comments on every distinct control class. Spec ids stay in comments / `.spec__note`, never inside the mock.

## Files

Lab: `attach-vocabulary.html` (S1 — image tile, file well, states, one-row rail + edge fades, rejected alternatives).
`attach.js` is required on every page that renders a strip or transcript gallery.
Finals: composer (S2), drop-paste (S3), picker (S4), stack (S5), files (S6), transcript (S7), refusals (S8), busy (S9).
`index.html` maps finals × labs.
