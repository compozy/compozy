---
id: RT-session-transcript-whitespace
area: RT
title: Session transcript preserves whitespace between streamed Markdown chunks
persona: Session operator
journey: J-14
expected: A split Mermaid fence renders only diagram syntax as code; the following Markdown heading and paragraph render outside it before and after reload. A version-1 persisted transcript reprojects affected assistant entries without changing event bytes or message identity.
entry_points: web session window transcript; session transcript REST
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: web/e2e/__tests__/session-transcript-whitespace.spec.ts; internal/store/sessiondb/transcript_projection_upgrade_test.go; docs/qa/reports/2026-09-23-session-transcript-whitespace.md
last_report: docs/qa/reports/2026-09-23-session-transcript-whitespace.md
overlaps: ET-web-session-transcript-calm-grammar, RT-022
---

Issue #669: verify newline-only and indentation-only chunks emitted between nonblank text chunks.
The isolated browser runtime uses a mock agent that splits a Mermaid fence, a Markdown heading,
and a paragraph across separate chunks. Check the live transcript, reload, and inspect the same
elements. The version-1 upgrade regression uses a temporary session database and verifies stable
event bytes, message identity, generation, and unrelated entries.

QA 2026-09-23: the isolated browser run passed after the reply and after reload. The backend
regression passed for version-1 repair, including whitespace-only chunks and padded raw text.

Issue #872: send a user message with a single line break, a blank line, and a two-item list. Verify that the second line is visually below the first and that both list items remain distinct before and after reload. Move the pointer outside the session and verify both user and assistant message actions remain visible. Owning browser coverage: `session-transcript-whitespace.spec.ts`.

QA 2026-10-21 (#872): PASS in the isolated browser runtime with the real daemon and ACP mock provider. Authored line layout, both list items, and user/assistant actions were verified before and after reload; existing streamed Markdown assertions also passed.

Issue #967: type the message the way an operator does: a first line, Shift+Enter, a second line that ends in a URL typed one character at a time, a blank line, then a pasted nested list (`- first item`, `- second item`, `  - nested item`). While typing, the composer decorates the whole URL as one link with the globe affordance, not only its first valid prefix. After send and after reload, the user bubble shows the second line below the first through a real line break, the URL as a globe link that opens in a new tab with `noopener noreferrer`, and the nested list with no blank lines between its items. Repeat with a skill command, an `@` mention, and an attachment: each keeps the same layout. Owning browser coverage: `session-transcript-whitespace.spec.ts`. Owning unit coverage: `StreamMarkdown` authored mode (`packages/ui`), the SessionThread user-message variants and composer URL suite, and the rewind-draft runtime-provider case.

QA 2026-10-24 (#967): PASS in the isolated browser runtime with the real daemon and the ACP mock provider. The composer link covered the full typed URL. Line breaks, the link, and the nested list rendered correctly before and after reload, including a message that combined a skill chip and a file mention. The prompt from the original report was confirmed in the daemon store as single-line authored text, so it showed no newline loss.
