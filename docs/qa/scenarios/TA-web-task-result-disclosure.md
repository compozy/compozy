---
id: TA-web-task-result-disclosure
area: TA
title: Inspect and copy a large task result without loading it eagerly
persona: Cora
journey: J-complete-partial-loop
expected: Task Overview and task-run detail show a closed result disclosure without fetching external bytes, opening renders one bounded 16 KiB page with clear loading, retry, and paging states, and Copy result reconstructs and decodes the complete UTF-8 value exactly once.
entry_points: Web task Overview; Web task-run detail
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260831-181713-956331-lab/qa-artifacts/qa/web-result-closed.png; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260831-181713-956331-lab/qa-artifacts/qa/web-result-expanded-page.png; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260831-181713-956331-lab/qa-artifacts/qa/web-result-network-copy.txt
last_report: docs/qa/reports/2026-08-31-loop-result-fix.md
overlaps: TA-task-run-result-paging; ET-tool-result-artifact-recovery
---

Open the same completed task from Overview and task-run detail. Confirm the large result remains closed by default, the page stays responsive, only one bounded page is visible at a time, previous/next navigation is truthful, and Copy result reports progress before producing the exact complete value. Refresh and deep-link back to the run to confirm the descriptor remains durable.

QA impact 2026-08-31: new bounded Web disclosure and sequential copy behavior.

QA 2026-08-31: the deep-linked task Overview showed a closed 71,694-byte result with no `/result` request. Opening fetched offset 0 only; paging rendered bytes 16,385–32,768 in the bounded code viewport; Copy result fetched all five 16 KiB pages and announced `Copied result`. Browser errors were empty.

QA impact 2026-09-17 (issue #653): a string task result renders through `<Markdown>` on the reading tier,
so it now takes the prose heading ladder (H1 22 px, H2 18 px, heading top margins up to 32 px), a 14 px
paragraph rhythm, accent-strong underlined links, and framed tables. Disclosure, paging, and copy
behaviour are untouched. Not inspected in Storybook and not walked: confirm the collapsed preview
(`max-h-44`) still shows useful body text when a result opens with headings, and that the expanded
result reads comfortably.
