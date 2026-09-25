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
