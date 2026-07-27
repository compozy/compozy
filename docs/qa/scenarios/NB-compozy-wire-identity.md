---
id: NB-compozy-wire-identity
area: NB
title: Expose only the Compozy wire identity
persona: Ada
journey: J-validate-compozy-hard-cut
expected: A fresh runtime reports and persists compozy-network/v0, uses compozy.runtime for its runtime peer, emits only the eight compozy.* capability/workflow/handoff extension keys, derives stable direct-room ids from the Compozy prefix, and contains no prior wire identifier in HTTP, UDS, CLI, native-tool, event, or protocol-document output.
entry_points: compozy network status/send/inbox -o json; HTTP and UDS network routes; compozy__network_*; Web Loop DSL; /protocol
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: NB-020; NB-run-bounded-live-collaboration; LP-023
---

QA impact 2026-07-26: the network protocol, runtime peer, extension keys,
soul/heartbeat digests, direct-room derivation, RFC filenames, and Loop DSL version
received one atomic hard cut. Planning flag only; execute from a fresh runtime because
persisted identities are intentionally invalidated and have no compatibility reader.
