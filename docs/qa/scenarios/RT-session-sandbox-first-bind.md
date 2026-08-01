---
id: RT-session-sandbox-first-bind
area: RT
title: Preserve sandbox policy until the first prompt binds the runtime
persona: Ada
journey: J-15
expected: A logical session created in a workspace with an explicit sandbox profile starts no provider before its first prompt, then binds that prompt in the selected sandbox exactly once and exposes matching session and transcript state through fresh public reads.
entry_points: `compozy session new`; `compozy session prompt`; HTTP and UDS session detail and transcript reads
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-extension-boundary-20260801-140242-564687-lab/qa-artifacts/qa/boundary-verification.json; /Users/pedronauck/dev/qa-labs/compozy-session-extension-boundary-20260801-140242-564687-lab/qa-artifacts/qa/provider-attempt.json; /Users/pedronauck/dev/qa-labs/compozy-session-extension-boundary-20260801-140242-564687-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-01-session-extension-boundary.md
overlaps: RT-010; RT-session-cwd-resume; RT-session-prompt-idempotency
---

This scenario owns the deferred-start boundary between durable logical session creation and the first
provider-backed prompt. The workspace's immutable sandbox selection must survive that boundary; an
absent runtime instance before the first prompt is not permission to infer `sandbox = none`.

Walk with a real provider and a real configured sandbox. Confirm creation returns before provider
startup, submit one prompt with durable identities, retry it once with the same identities, then use
fresh CLI and HTTP reads to prove one authored turn, one provider response, and the selected sandbox.

2026-08-01 isolated walk: CLI creation returned `runtime.status=unbound` for workspace registration
`ws_fcd6ecd9076c58c6` with `sandbox_ref=local`. The first live Codex prompt completed turn
`turn-abd6e5c31301aee8`; its exact retry returned `replayed=true`. A fresh HTTP session read reported
`transition=initial_bind` and a prepared `local` sandbox, while the transcript contained exactly one
authored message (`msg-qa-sandbox-first-bind-1`) and one `SANDBOX-READY` response. Teardown was clean.
