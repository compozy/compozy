---
id: NB-long-bridge-replies
area: NB
title: Deliver long bridge replies safely
persona: Omar
journey: J-deliver-long-formatted-reply
expected: A terminal reply above the platform cap is delivered as ordered numbered messages whose wire bodies stay within Slack 40,000 UTF-16 code units, Telegram 4,096 UTF-16 code units, Discord 2,000 Unicode code points, Teams 28,000 Unicode code points, Google Chat 32,000 UTF-8 bytes, or WhatsApp 4,096 Unicode code points; edit-capable cumulative previews stay in one mutable message until terminal continuations materialize; after an exhausted typed provider retry, same-process redelivery resumes at the first unsent chunk without duplicating the acknowledged prefix; platform markup stays readable; and the delivery acknowledgement points to the final remote message.
entry_points: Public bridge delivery through Slack; Telegram; Discord; Teams; Google Chat; WhatsApp adapters
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/notes/bridge-charter-results.json
last_report: docs/qa/reports/2026-07-12-hermes-bridge.md
overlaps: NB-039; NB-bridge-tool-progress
---

An operator or teammate receives a complete long agent reply without provider rejection, duplicate streaming continuations, or broken platform markup.

Added by the Hermes bridge Task 02 impact flag. Task 09 assigned it to `J-deliver-long-formatted-reply` and `CH-long-provider-replies`; Task 10 owns execution. Planning flag only; no QA session ran.

QA 2026-07-13: the adversarial shared corpus passed all six provider wire owners under race with provider-specific units, ordered continuations, fence repair, dialect handling, and terminal acknowledgements. This is a qualified deterministic-provider verdict.

Provider limits covered by this scenario are Slack 40,000 UTF-16 code units, Telegram 4,096 UTF-16 code units, Discord 2,000 Unicode code points, Teams 28,000 Unicode code points, Google Chat 32,000 UTF-8 bytes, and WhatsApp 4,096 Unicode code points.

Phase D impact flag 2026-07-13: all six chat adapters now apply typed retry/backoff to primary delivery operations, retain only the successfully acknowledged chunk prefix, and resume at the first unsent chunk after exhausted retries. Slack wire measurement changed from rune count to 40,000 UTF-16 code units. Status reset to `untested`; historical deterministic-provider evidence remains intact. No QA retest ran.
