---
id: ET-session-mention-folder-quoted
area: ET
title: Mention a folder and a path with spaces
persona: Théo
journey:
expected: Picking a folder inserts a chip ending in `/` and sends `@web/src/`; picking `docs/My Notes.md` sends `@"docs/My Notes.md"`; the agent receives a folder resource_link whose name and uri end in `/` and a file link whose name keeps the space and whose uri percent-encodes it; mentioning the same file twice shows two chips and delivers one link.
entry_points: web session window composer; compozy session prompt; compozy session events --json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-mention-file
---

US-002.AC-2/AC-5, US-002.EC-1/EC-2, US-005.AC-2, US-005.EC-2/EC-4. Prepare a working directory with
`web/src/` and `docs/My Notes.md`. In the composer type `Look at @web/s`, pick the `src` folder row:
the chip reads `src/`. Continue with `and @My`, pick `My Notes.md`: the chip reads `My Notes.md`
without quotes. Add `@My` again and pick the same file a second time.

Send. The transcript bubble shows three chips. The delivered prompt has the text
`Look at @web/src/ and @"docs/My Notes.md" … @"docs/My Notes.md"`, then exactly two `resource_link`
blocks in first-mention order: `{name: "web/src/", uri: "file:///…/web/src/"}` and
`{name: "docs/My Notes.md", uri: "file:///…/docs/My%20Notes.md"}`. Repeat through
`compozy session prompt` with a typed `@web/src` (no trailing slash) and confirm it still resolves as
a folder.
