---
id: RT-model-catalog-cold-open
area: RT
title: Cold model catalog opens without waiting for provider probes
persona: Sol
journey: J-17
expected: On the first selector open after daemon start, the persisted model catalog becomes usable without waiting for live provider discovery. Provider probes warm the catalog in daemon-owned background work, refreshed entries appear later, and shutdown joins or cancels that work cleanly.
entry_points: onboarding default-model selector; agent runtime selector; session composer RuntimeSelector
qa_status: pass
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
