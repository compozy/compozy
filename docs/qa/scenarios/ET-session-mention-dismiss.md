---
id: ET-session-mention-dismiss
area: ET
title: Dismiss or ignore the @ picker and the literal text is sent as typed
persona: Théo
journey:
expected: Esc closes the picker and keeps it closed for that `@` occurrence while typing continues after it; a new `@` after whitespace reopens it; Enter with no rows sends the prompt as typed; a space right after `@query` closes the picker and keeps the text; during IME composition ↑/↓, Enter, and Tab go to the IME candidate window, not the picker; in every leg the sent text is exactly what was typed and no mention is delivered for a dismissed token.
entry_points: web session window composer; compozy session events --json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-mention-file
---

US-004.AC-1/2/3/4, US-004.EC-1. Use a real session whose working directory has no file named
`Injectable` or `decorator`.

Esc leg: type `Use @Inj`; the picker opens. Press Esc: it closes and the text still reads `Use @Inj`.
Keep typing `ectable on the class` with no space after `@Inj`: the picker stays closed for that
occurrence. Type ` and @` (space first): the picker opens again for the new `@`. Press Esc again.

Enter-with-no-rows leg: type `Rename @nothing-matches-this` until both groups read "No matching
files" and "No matching sessions", then press Enter. The prompt is sent and the bubble shows
`Rename @nothing-matches-this` as plain text.

Space leg: type `Ping @clie` (the picker lists `client_prompt.go`), then type a space and `please`. The
picker closes, no chip is inserted, and the text reads `Ping @clie please`. Send it and confirm the
bubble shows the literal text.

IME leg: switch the macOS input source to Japanese – Romaji (or Pinyin – Simplified). Type `@` and
then a composing query such as `kaku`, so the IME candidate window is open while the picker is open.
↑/↓ move the IME candidate selection, Tab/Enter commit the candidate, and the picker's highlight
neither moves nor picks a row while composition is in progress. After the candidate is committed,
↑/↓ and Enter act on the picker again. Switch back to the previous input source.

In `compozy session events <id> --json`, every delivered prompt from these legs has the typed text
only: no `resource_link`, no `<compozy_session_references>` block, and no `mentions` on the user
message.
