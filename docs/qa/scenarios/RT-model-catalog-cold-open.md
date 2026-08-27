---
id: RT-model-catalog-cold-open
area: RT
title: Cold model catalog opens without waiting for provider probes
persona: Sol
journey: J-17
expected: On the first selector open after daemon start, persisted rows are immediately usable; provider probes refresh in daemon-owned background work after the five-minute TTL and on periodic ticks, newly advertised models appear without a code update, failed refresh keeps rows stale, and shutdown joins or cancels work cleanly.
entry_points: onboarding default-model selector; agent runtime selector; session composer RuntimeSelector
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/runtime-ui-proof.md; /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/font-csp-onboarding.png
last_report: docs/qa/reports/2026-08-27-runtime-ui-regressions.md
overlaps: ET-web-runtime-selector-minimal-slider; RT-068; RT-072
---

QA impact 2026-08-27: catalog reads no longer perform sequential live provider probes on the request
path. The daemon returns persisted entries immediately and owns the asynchronous refresh lifecycle.

QA impact 2026-08-27: live catalogs now persist option descriptors and private bindings by execution
context and refresh on TTL plus periodic cadence. Reset for cold read, stale failure, new-row, and clean
shutdown evidence.
