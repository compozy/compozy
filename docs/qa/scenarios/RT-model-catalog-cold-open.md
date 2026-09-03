---
id: RT-model-catalog-cold-open
area: RT
title: Cold model catalog opens and repairs missing provider rows
persona: Sol
journey: J-17
expected: On the first selector open after daemon start, persisted rows are immediately usable; if an allowed provider has no rows, Web requests one aggregate refresh and rereads the catalog once, while provider probes remain daemon-owned background work. Newly advertised models appear without a code update, failed refresh keeps rows stale, and shutdown joins or cancels work cleanly.
entry_points: onboarding default-model selector; agent runtime selector; session composer RuntimeSelector
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/runtime-ui-proof.md; /Users/pedronauck/dev/qa-labs/compozy-cursor-onboarding-runtime-defaults-retest-20260828-171621-219738-lab/qa-artifacts/qa/notes/cursor-defaults-retest-evidence.md; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/screenshots/onboarding-runtime.png; docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
last_report: docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
overlaps: ET-web-runtime-selector-minimal-slider; RT-068; RT-072
---

QA impact 2026-08-27: catalog reads no longer perform sequential live provider probes on the request
path. The daemon returns persisted entries immediately and owns the asynchronous refresh lifecycle.

QA impact 2026-08-27: live catalogs now persist option descriptors and private bindings by execution
context and refresh on TTL plus periodic cadence. Reset for cold read, stale failure, new-row, and clean
shutdown evidence.

QA 2026-08-27: after publishing and then failing a same-source synthetic Cursor model feed, a fresh
daemon process returned the persisted `available_stale` row in 0.02 seconds. The read did not wait for
the failing provider probe, and a later successful forced refresh replaced the stale generation.

QA impact 2026-08-28: the first Web read now performs one deduplicated aggregate refresh and reread
when a configured provider such as Cursor has no persisted rows. Reset for a fresh-onboarding walk
that proves the models appear without pressing Reload.

QA 2026-08-28: pass. A fresh isolated onboarding opened the model picker once and immediately
showed Cursor Agent, Grok 4.5, Grok 4.6, and GPT-5.6 Terra without pressing catalog refresh.
