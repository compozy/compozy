---
id: RT-model-catalog-cold-open
area: RT
title: Cold model catalog opens without waiting for provider probes
persona: Sol
journey: J-17
expected: On the first selector open after daemon start, persisted rows are immediately usable; provider probes refresh in daemon-owned background work after the five-minute TTL and on periodic ticks, newly advertised models appear without a code update, failed refresh keeps rows stale, and shutdown joins or cancels work cleanly.
entry_points: onboarding default-model selector; agent runtime selector; session composer RuntimeSelector
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/runtime-ui-proof.md; /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/font-csp-onboarding.png;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/live-model-refresh-cold-open.json
last_report: docs/qa/reports/2026-08-27-acp-runtime-catalog.md
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
